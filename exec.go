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

func CheckProcessFromPid(pidfile string) bool {
	var file *os.File

	if _, err := os.Stat(pidfile); os.IsNotExist(err) {
		if file, err = os.Create(pidfile); err != nil {
			return err
		}
	} else {
		if file, err = os.OpenFile(pidfile, os.O_RDWR, 0); err != nil {
			return err
		}
		pidstr := make([]byte, 8)

		n, err := file.Read(pidstr)
		if err != nil {
			return err
		}

		if n > 0 {
			pid, err := strconv.Atoi(string(pidstr[:n]))
			if err != nil {
				fmt.Printf("err: %s, overwriting pidfile", err)
			}

			process, _ := os.FindProcess(pid)
			if err = process.Signal(syscall.Signal(0)); err == nil {
				return fmt.Errorf("pid: %d is running", pid)
			} else {
				fmt.Printf("err: %s, cleanup pidfile", err)
			}

			if file, err = os.Create(pidfile); err != nil {
				return err
			}

		}

	}

	pid := strconv.Itoa(os.Getpid())
	fmt.Fprintf(file, "%s", pid)
	return nil
}
