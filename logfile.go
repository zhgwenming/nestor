// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

// +build go1.4

package nestor

import (
	"fmt"
	"io"
	"os"
)

type logFile struct {
	name   *string
	file   *os.File
	offset int64
}

func (l *logFile) Open() error {
	file, err := os.OpenFile(*l.name, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		return err
	}

	l.file = file

	// mark the end offset
	offset, err := file.Seek(0, os.SEEK_END)
	if err != nil {
		l.offset = offset
	}

	return err
}

func (l *logFile) Dump(output io.Writer) error {
	buf := make([]byte, 1024)
	_, err := l.file.ReadAt(buf, l.offset)
	fmt.Fprintf(output, "daemon output:\n%s\n", buf)
	if err != io.EOF {
		fmt.Printf("\n\nLog file is too long, please go check directly")
	}
	return nil
}
