// Copyright 2019 Yegor Myskin. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tailor

import (
	"unsafe"
)

// Line represents a returned line from the tailed file.
type Line struct {
	fileName string
	line     []byte
}

// FileName returns the file name of the line.
func (l Line) FileName() string {
	return l.fileName
}

// String returns an untrimmed string.
func (l Line) String() string {
	return *(*string)(unsafe.Pointer(&l.line))
}

// StringTrimmed returns a trimmed string.
func (l Line) StringTrimmed() string {
	trimmedString := l.BytesTrimmed()
	return *(*string)(unsafe.Pointer(&trimmedString))
}

// Bytes returns a line.
func (l Line) Bytes() []byte {
	return l.line
}

// BytesTrimmed trims the line from the \r and \n sequences. Not unicode safe.
func (l Line) BytesTrimmed() []byte {
	// inline optimization with goto instead of for
Loop:
	if len(l.line) == 0 {
		return l.line
	}

	last := l.line[len(l.line)-1]
	if last == '\n' || last == '\r' || last == ' ' {
		l.line = l.line[:len(l.line)-1]
	} else {
		return l.line
	}
	goto Loop
}
