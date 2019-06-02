package tailor

import (
	"bytes"
	"testing"
)

func TestLine(t *testing.T) {
	b := []byte("")
	l := Line{
		line:     b,
		fileName: "file.txt",
	}

	if l.FileName() != "file.txt" {
		t.Fail()
		return
	}

	if !bytes.Equal(b, l.Bytes()) {
		t.FailNow()
		return
	}

	if !bytes.Equal(b, l.BytesTrimmed()) {
		t.FailNow()
		return
	}

	if l.String() != "" {
		t.FailNow()
		return
	}

	if l.StringTrimmed() != "" {
		t.FailNow()
		return
	}

	l.line = []byte("abc\nde\n\r \n")

	if !bytes.Equal([]byte("abc\nde\n\r \n"), l.Bytes()) {
		t.FailNow()
		return
	}

	if !bytes.Equal([]byte("abc\nde"), l.BytesTrimmed()) {
		t.FailNow()
		return
	}

	if l.String() != "abc\nde\n\r \n" {
		t.FailNow()
		return
	}

	if l.StringTrimmed() != "abc\nde" {
		t.FailNow()
		return
	}

	l.line = []byte("\n")

	if !bytes.Equal([]byte(""), l.BytesTrimmed()) {
		t.FailNow()
		return
	}
}
