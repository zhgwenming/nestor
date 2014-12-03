// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package nestor

import (
	"fmt"
)

const (
	ErrCode = 1
)

var (
	_ = fmt.Printf
)

type NestorError struct {
	errcode int
}
