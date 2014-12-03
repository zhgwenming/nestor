// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package nestor

import (
	"os/exec"
	"sync"
)

type Cmd struct {
	sync.Mutex
	*exec.Cmd
}

func (c *Cmd) Command(name string, arg ...string) error {
	c.Cmd = exec.Command(name, arg...)
	return nil
}
