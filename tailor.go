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

type Tailor struct {
	fileName string
	file     *os.File

	opts options

	// stats
	lastPos  int64
	lastSize int64
	lag      int64

	lines chan Line
	errs  chan error

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
func (t *Tailor) Run(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&t.working, 0, 1) {
		return errors.New("already working")
	}

	failFinalizer := func() {
		if t.file != nil {
			_ = t.file.Close()
		}
		atomic.StoreInt32(&t.working, 0)
	}

	err := t.openFile(t.opts.runOffset, t.opts.runWhence)
	if err != nil {
		failFinalizer()
		return errors.Wrap(err, "can't open file for tailing")
	}

	t.readLoop(ctx)

	return nil
}

// readLoop starts goroutine, which reads the given file and send to the line chan tailed strings.
func (t *Tailor) readLoop(ctx context.Context) {
	t.lines = make(chan Line)
	t.errs = make(chan error)

	go func() {
		defer func() {
			if t.file != nil {
				err := t.file.Close()
				if err != nil {
					t.errs <- errors.Wrap(err, "error closing file")
				}
			}
			close(t.lines)
			close(t.errs)

			if t.opts.rateLimiter != nil {
				t.opts.rateLimiter.Close()
			}

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
					t.errs <- errors.Wrap(err, "error getting file status")
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
				} else {
					t.errs <- errors.Wrap(err, "error reading line")
					return
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
			if len(line) == 0 && err == io.EOF {
				isSameFile, err := t.isFileStillTheSame()
				if err != nil {
					t.errs <- errors.Wrap(err, "error checking that file is the same")
					return
				}

				if !isSameFile {
					err := t.file.Close()
					if err != nil {
						t.errs <- errors.Wrap(err, "error closing current file")
					}

					err = t.openFile(t.opts.reopenOffset, t.opts.reopenWhence)
					if err != nil {
						t.errs <- errors.Wrap(err, "error reopening file")
						return
					}

					r = bufio.NewReaderSize(t.file, t.opts.bufioReaderPoolSize)
					pollerTimeout = t.opts.pollerTimeout

					continue
				}

				pollerTimeout = t.exponentialSleep(pollerTimeout, 5*time.Second)
				continue
			}
			if err != nil && err != io.EOF {
				t.errs <- errors.Wrap(err, "error reading line")
				return
			}

			pollerTimeout = t.opts.pollerTimeout

			if t.opts.rateLimiter == nil || t.opts.rateLimiter.Allow() {
				line := Line{
					line:     line,
					fileName: t.fileName,
				}

				if t.opts.leakyBucket {
					select {
					case t.lines <- line:
					default:
					}
					continue
				}

				t.lines <- line
			}
		}
	}()
}

// Lines returns chanel of read lines.
func (t *Tailor) Lines() chan Line {
	return t.lines
}

// Errors returns chanel of errors, associated with reading files.
func (t *Tailor) Errors() chan error {
	return t.errs
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

		if err != nil {
			return false, errors.Wrap(err, "error stating maybe new file by name")
		}

		break
	}

	currentFileInfo, err := t.file.Stat()
	if err != nil {
		return false, errors.Wrap(err, "error stating current file")
	}

	return os.SameFile(currentFileInfo, maybeNewFileInfo), nil
}

// FileName returns the name of the tailed file.
func (t Tailor) FileName() string {
	return t.fileName
}

// Lag returns approximate lag, updater per interval.
func (t Tailor) Lag() int64 {
	return atomic.LoadInt64(&t.lag)
}

// openFile opens the file for reading, seeks to the beginning of the line at opts.*offset and updates the lag.
func (t *Tailor) openFile(offset int64, whence int) (err error) {
	t.file, err = os.Open(t.fileName)
	if err != nil {
		return errors.Wrap(err, "error opening file")
	}

	err = t.seekToLineStart(offset, whence)
	if err != nil {
		return errors.Wrap(err, "error seeking to line start")
	}

	err = t.updateFileStatus()
	if err != nil {
		return errors.Wrap(err, "error updating file status")
	}

	return nil
}

// seekToLineStart seeks the cursor at the beginning of a line at offset. Internally this function uses a buffer
// to find the beginning of a line. If the byte at offset equals \n, so the next line will be selected.
func (t *Tailor) seekToLineStart(offset int64, whence int) error {
	const (
		bufSize int64 = 256
	)

	initialOffset, err := t.file.Seek(offset, whence)
	if initialOffset == 0 {
		return nil
	}
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		return err
	}

	min := func(a, b int64) int64 {
		if a < b {
			return a
		}
		return b
	}

	var current int64 = 0
Loop:
	for {
		current += min(bufSize, initialOffset-current)
		buf := make([]byte, min(current, bufSize))

		n, err := t.file.ReadAt(buf, initialOffset-current)
		if err != nil && err != io.EOF {
			return err
		}
		buf = buf[:n]

		current -= int64(n)
		for i := int64(len(buf)) - 1; i >= 0; i-- {
			if buf[i] == '\n' {
				break Loop
			}
			current++
		}
		if initialOffset-current == 0 {
			break
		}
	}

	_, err = t.file.Seek(-current, io.SeekCurrent)
	if err == io.EOF {
		err = nil
	}

	return err
}

// updateFileStatus update a current seek from the file an an actual file size.
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
