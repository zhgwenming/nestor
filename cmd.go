// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package nestor

import (
	"os/exec"
)

type Cmd struct {
	*exec.Cmd
}

func (c *Cmd) Run() {
	c.Cmd.Run()
}
