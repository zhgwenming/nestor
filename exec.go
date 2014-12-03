// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package nestor

import (
	"errors"
	"os"
	"os/exec"
	"sync"
)

type Cmd struct {
	sync.Mutex
	*exec.Cmd
	name string
	arg  []string
	proc *os.Process
	done bool
}

func NewCmd(name string, arg ...string) *Cmd {
	cmd := &Cmd{name: name, arg: arg}
	return cmd
}

func (c *Cmd) Done() {
	c.Lock()
	defer c.Unlock()

	c.done = true
}

func (c *Cmd) Kill(sig os.Signal) {
	c.Lock()
	defer c.Unlock()

	c.done = true
	c.Process.Signal(sig)
}

// Start Under the protection of mutex
func (c *Cmd) Start() error {
	cmd := exec.Command(c.name, c.arg...)
	c.Cmd = cmd

	c.Lock()
	defer c.Unlock()

	if c.done {
		err := errors.New("Explicit Closed")
		return err
	}

	if err := c.Cmd.Start(); err != nil {
		return err
	}

	c.proc = c.Cmd.Process
	return nil
}

func (c *Cmd) Run() error {
	if err := c.Start(); err != nil {
		return err
	}

	return c.Cmd.Wait()
}
