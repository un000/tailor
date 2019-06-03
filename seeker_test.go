// Copyright 2019 Yegor Myskin. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tailor

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestNewLineFinder(t *testing.T) {
	var tests = []struct {
		content         string
		offsetFromStart int64
		res             string
	}{
		{"", 0, ""},
		{"\n", 0, "\n"},
		{"\n\n", 1, "\n"},
		{"\n\na\n", 3, "a\n"},
		{"a", 0, "a"},
		{"a\n", 0, "a\n"},
		{"abc", 2, "abc"},
		{"abc\n", 2, "abc\n"},
		{"a\nb", 2, "b"},
		{"a\nb\n", 2, "b\n"},
		{"aaaaa\nbbbbbbbb\n", 4, "aaaaa\n"},
		{"aaaaa\nbbbbbbbb\n", 10, "bbbbbbbb\n"},
		{strings.Repeat("a", 300), 280, strings.Repeat("a", 300)},
		{strings.Repeat("a", 300) + "\n", 280, strings.Repeat("a", 300) + "\n"},
		{strings.Repeat("a", 100) + "\n" + strings.Repeat("a", 200), 280, strings.Repeat("a", 200)},
	}

	const file = "./tst"
	defer os.Remove(file)

	for i, data := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			err := ioutil.WriteFile(file, []byte(data.content), os.ModePerm)
			if err != nil {
				t.Error(err)
				return
			}

			f := New(file)
			err = f.openFile(data.offsetFromStart, io.SeekStart)
			if err != nil {
				t.Errorf("[%d] error executing: %s, data: %+v", i, err, data)
				return
			}

			r := bufio.NewReader(f.file)
			line, err := r.ReadString('\n')
			if err != nil && err != io.EOF {
				t.Errorf("[%d] error reading line: %s, data: %+v", i, err, data)
				return
			}

			if line != data.res {
				t.Errorf("[%d] actual: '%s', want: '%s', data: %+v", i, line, data.res, data)
				return
			}
		})
	}
}
