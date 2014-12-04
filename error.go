// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package nestor

import (
	"fmt"
)

type ErrorCode int

const (
	OK ErrorCode = iota
	ErrPanic
	ErrFileTooLarge
)

var (
	_ = fmt.Printf
)

type Error struct {
	ErrorCode ErrorCode
}

func newError(e ErrorCode) *Error {
	return &Error{e}
}

// todo
func (e *Error) Error() string {
	return ""
}
