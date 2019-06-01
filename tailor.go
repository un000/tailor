// Copyright 2019 Yegor Myskin. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tailor

import (
	"bufio"
	"context"
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
)

var (
	ErrFileNotExists = os.ErrNotExist
)

type Tailor struct {
	fileName string
	file     *os.File

	opts options

	lastPos  int64
	lastSize int64
	lag      int64

	working int32
}

// New prepares the instance of Tailor. It mergers default options with the given options.
func New(filename string, opts ...Option) *Tailor {
	t := &Tailor{
		fileName: filename,
	}

	for _, p := range [][]Option{withDefaultOptions(), opts} {
		for _, o := range p {
			o(&t.opts)
		}
	}

	return t
}

// Run starts the tailing procedure.
// 1. Opens a file in RO mode and seeks to the newest position by default(can be changed in options).
// 2. Then reads the file line by line and produces lines and errors through the channels.
// If the file has been logrotated, Tailor will follow the first file to the end and after reopen it.
// If error happens file will be closed.
// Tailor makes an exponential sleep to reduce stat syscalls.
func (t *Tailor) Run(ctx context.Context) (<-chan Line, <-chan error, error) {
	if !atomic.CompareAndSwapInt32(&t.working, 0, 1) {
		return nil, nil, errors.New("already working")
	}

	finalizer := func() {
		if t.file != nil {
			_ = t.file.Close()
		}
		atomic.StoreInt32(&t.working, 0)
	}

	var err error
	t.file, err = os.Open(t.fileName)
	if err != nil {
		finalizer()
		return nil, nil, errors.Wrap(err, "can't open file for tailing")
	}

	_, err = t.file.Seek(t.opts.runOffset, t.opts.runWhence)
	if err != nil {
		finalizer()
		return nil, nil, errors.Wrapf(err, "error seeking file %s", t.fileName)
	}

	err = t.seekToLineStart()
	if err != nil {
		finalizer()
		return nil, nil, errors.Wrapf(err, "error seeking to the line beginning %s", t.fileName)
	}

	{
		err := t.updateFileStatus()
		if err != nil {
			finalizer()
			return nil, nil, errors.Wrapf(err, "error getting file size %s", t.fileName)
		}
	}

	lines, errs := t.readLoop(ctx)

	return lines, errs, nil
}

// readLoop starts goroutine, which reads the given file and send to the line chan tailed strings.
func (t *Tailor) readLoop(ctx context.Context) (chan Line, chan error) {
	lines := make(chan Line)
	errs := make(chan error)

	go func() {
		defer func() {
			if t.file != nil {
				err := t.file.Close()
				if err != nil {
					errs <- errors.Wrap(err, "error closing file")
				}
			}
			close(lines)
			close(errs)
			atomic.StoreInt32(&t.working, 0)
		}()

		r := bufio.NewReaderSize(t.file, t.opts.bufioReaderPoolSize)
		lagReporter := time.NewTicker(t.opts.updateLagInterval)
		defer lagReporter.Stop()

		pollerTimeout := t.opts.pollerTimeout
		for {
			select {
			case <-ctx.Done():
				return
			case <-lagReporter.C:
				err := t.updateFileStatus()
				if err != nil {
					errs <- errors.Wrap(err, "error getting file status")
					break
				}
			default:
			}

			var (
				part []byte
				line []byte
				err  error
			)

			// a Line can be read partially
			// so wait until the new one comes
			// if the line wasn't read after 5 tries return it
			for i := 0; i < 5; i++ {
				part = part[:0]
				part, err = r.ReadBytes('\n')
				if err == nil || err == io.EOF {
					// if Line is new and finished, don't allocate buffer, just copy ref
					if len(line) == 0 && len(part) > 0 && part[len(part)-1] == '\n' {
						line = part
						break
					}
				}
				line = append(line, part...)

				if len(line) == 0 && err == io.EOF {
					break
				}

				if line[len(line)-1] == '\n' {
					break
				}

				pollerTimeout = t.exponentialSleep(pollerTimeout, time.Second)
			}

			// check that logrotate swapped the file
			if err == io.EOF && len(line) == 0 {
				isSameFile, err := t.isFileStillTheSame()
				if err != nil {
					errs <- err
					return
				}

				if !isSameFile {
					err := t.file.Close()
					if err != nil {
						errs <- errors.Wrap(err, "error closing current file")
					}

					t.file, err = os.Open(t.fileName)
					if err != nil {
						if os.IsNotExist(err) {
							errs <- ErrFileNotExists
							return
						}
						errs <- errors.Wrap(err, "error reopening file")
						return
					}

					r = bufio.NewReaderSize(t.file, t.opts.bufioReaderPoolSize)
					pollerTimeout = t.opts.pollerTimeout
					err = t.updateFileStatus()
					if err != nil {
						errs <- errors.Wrap(err, "error getting file status")
						return
					}
					err = t.seekToLineStart()
					if err != nil {
						errs <- errors.Wrap(err, "error seeking to the line beginning")
						return
					}

					continue
				}

				pollerTimeout = t.exponentialSleep(pollerTimeout, 5*time.Second)
				continue
			}
			if err != nil && err != io.EOF {
				errs <- errors.Wrapf(err, "error reading Line")
				continue
			}

			pollerTimeout = t.opts.pollerTimeout

			lines <- Line{
				line:     line,
				fileName: t.fileName,
			}
		}
	}()

	return lines, errs
}

// exponentialSleep sleeps for pollerTimeout and returns new exponential grown timeout <= maxWait.
func (t *Tailor) exponentialSleep(pollerTimeout time.Duration, maxWait time.Duration) time.Duration {
	time.Sleep(pollerTimeout)
	// use exponential poller duration to reduce the load
	if pollerTimeout < maxWait {
		return pollerTimeout * 2
	}

	return maxWait
}

// isFileStillTheSame checks that opened file wasn't swapped.
func (t *Tailor) isFileStillTheSame() (isSameFile bool, err error) {
	var maybeNewFileInfo os.FileInfo

	// maybe the current file is being rotated with a small lag, check it for some tries
	for i := 0; i < 2; i++ {
		maybeNewFileInfo, err = os.Stat(t.fileName)
		if os.IsNotExist(err) {
			time.Sleep(time.Second)
			continue
		}
		if err == nil {
			break
		}
	}
	if err != nil {
		return false, errors.Wrap(err, "error stating maybe new file by name")
	}

	currentFileInfo, err := t.file.Stat()
	if err != nil {
		return false, errors.Wrap(err, "error stating current file")
	}

	return os.SameFile(currentFileInfo, maybeNewFileInfo), nil
}

// FileName returns the name of the tailed file.
func (t *Tailor) FileName() string {
	return t.fileName
}

// Lag returns approximate lag, updater per interval.
func (t *Tailor) Lag() int64 {
	return atomic.LoadInt64(&t.lag)
}

// seekToLineStart seeks the cursor at the beginning of a line at offset.
func (t *Tailor) seekToLineStart() error {
	bts := make([]byte, 1)

	offset, err := t.file.Seek(0, io.SeekCurrent)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}

	for offset != 0 {
		_, err = t.file.Read(bts)
		if err != nil && err != io.EOF {
			return err
		}

		b := bts[0]
		if b == '\n' {
			return nil
		}

		offset, err = t.file.Seek(-2, io.SeekCurrent)
		if err != nil {
			return err
		}
	}

	return nil
}

// updateFileStatus update a current seek from the file an an actual file size.
// If the byte at offset equals \n, so next line will be selected.
func (t *Tailor) updateFileStatus() (err error) {
	fi, err := t.file.Stat()
	if err != nil {
		return err
	}

	pos, err := t.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	size := fi.Size()
	atomic.StoreInt64(&t.lastPos, pos)
	atomic.StoreInt64(&t.lastSize, size)
	atomic.StoreInt64(&t.lag, size-pos)

	return nil
}
