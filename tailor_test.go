// Copyright 2019 Yegor Myskin. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tailor_test

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/un000/tailor"
)

func TestTailFileFromStart(t *testing.T) {
	const fileName = "./file_from_start"
	fileData := []byte(`1
2
3`)

	err := ioutil.WriteFile(fileName, fileData, os.ModePerm)
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(fileName)

	f := tailor.New(fileName, tailor.WithSeekOnStartup(0, io.SeekStart))
	if fileName != f.FileName() {
		t.Error("file name mismatch")
	}

	ctx, _ := context.WithTimeout(context.Background(), 2*time.Second)
	lines, errs, err := f.Run(ctx)
	if err != nil {
		t.Error(err)
	}

	var i = 1
	defer func() {
		if i != 4 {
			t.Error("not read to the end, last line:", i)
		}
	}()

	for ; i <= 3; i++ {
		select {
		case line, ok := <-lines:
			if !ok {
				return
			}

			if line.StringTrimmed() != strconv.Itoa(i) {
				t.Error(err)
			}
			t.Log(line.StringTrimmed())
		case err, ok := <-errs:
			if !ok {
				return
			}
			t.Error(err)
		}
	}
}
