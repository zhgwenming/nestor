// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package nestor

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
)

const (
	MaxPidFileSize = 8
)

type Cmd struct {
	sync.Mutex
	*exec.Cmd
	pidfile string
	name    string
	arg     []string
	done    bool
}

func NewCmd(name string, arg ...string) *Cmd {
	cmd := &Cmd{name: name, arg: arg}
	return cmd
}

func (c *Cmd) SetPidFile(pidfile string) {
	c.pidfile = pidfile
}

func (c *Cmd) Signal(sig os.Signal) {
	c.Lock()
	defer c.Unlock()

	c.done = true
	if c.Process != nil {
		c.Process.Signal(sig)
	}
}

// Kill is to signal the process with sig KILL
func (c *Cmd) Kill() {
	c.Signal(os.Kill)
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

	return nil
}

func (c *Cmd) Run() error {
	if err := c.Start(); err != nil {
		return err
	}

	return c.Cmd.Wait()
}

func (c *Cmd) TermRun() error {
	buf := make([]byte, 16)
	os.Stdin.Read(buf)

	if err := c.Start(); err != nil {
		return err
	}

	return c.Cmd.Wait()
}

func ReadPid(filename string) (int, error) {
	f, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	if fi, err := f.Stat(); err != nil {
		return 0, err
	} else {
		if size := fi.Size(); size >= MaxPidFileSize {
			return 0, newError(ErrFileTooLarge)
		}
	}

	// actuall read the file
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return 0, err
	}

	if pid, err := strconv.Atoi(string(buf)); err != nil {
		return 0, err
	} else {
		return pid, nil
	}

}

func CheckProcess(pid int) bool {
	process, _ := os.FindProcess(pid)
	if err := process.Signal(syscall.Signal(0)); err == nil {
		return true
	} else {
		return false
	}
}
