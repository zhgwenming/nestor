// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package nestor

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	ENV_SUPERVISOR = "__GO_SUPERVISOR_MODE"
	LOGIN_SHELL    = "/bin/bash"
)

var (
	DefaultSupervisor = NewSupervisor()
)

type Supervisor struct {
	*Daemon
	lsh  *Cmd
	cmds []*Cmd
}

func NewSupervisor() *Supervisor {
	d := NewDaemon()
	c := make([]*Cmd, 0, 4)
	lsh := NewCmd(LOGIN_SHELL, "-l")

	return &Supervisor{Daemon: d, lsh: lsh, cmds: c}
}

func (s *Supervisor) embededWorker() {
	cmd := s.Cmd

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err == nil {
		log.Printf("- Started worker as pid %d\n", cmd.Process.Pid)
	} else {
		log.Printf("error to start worker - %s\n", err)
		os.Exit(1)
	}

	for sig := range s.Signalc {
		log.Printf("monitor captured %v\n", sig)
		if sig == syscall.SIGCHLD {
			break
		}

		// only exit if we got a TERM signal
		if sig == syscall.SIGTERM {
			cmd.Process.Signal(sig)
			os.Exit(0)
		}
	}

	if err := cmd.Wait(); err != nil {
		log.Printf("worker[%d] exited with - %s\n", cmd.Process.Pid, err)
	}

}

func (s *Supervisor) supervise() {
	signal.Notify(s.Signalc,
		syscall.SIGCHLD)

	// process manager
	for {
		startTime := time.Now()
		s.embededWorker()
		for {
			endTime := time.Now()
			duration := endTime.Sub(startTime)
			seconds := duration.Seconds()

			// restart for every 5s
			if seconds > 5 {
				break
			} else {
				time.Sleep(time.Second)
			}
		}
		log.Printf("restarting worker\n")
	}
}

func (s *Supervisor) loginShell() {
	for {
		s.RunWait(s.lsh.TermRun)
	}
}

func (s *Supervisor) start() error {
	if len(s.cmds) == 0 {
		err := errors.New("no cmd specified")
		return err
	}

	// run all cmds as goroutines
	for _, c := range s.cmds {
		s.RunForever(c.Run)
	}

	// fork the foreground bash
	// more switch to controll this?
	if s.Foreground {
		go s.loginShell()
	}

	// wait to exit
	for sig := range s.Signalc {
		log.Printf("monitor captured %v\n", sig)

		// only exit if we got a TERM signal
		if sig == syscall.SIGTERM {
			// kill the login shell
			s.lsh.Signal(sig)

			// kill all the children
			for _, c := range s.cmds {
				c.Signal(sig)
			}
			os.Exit(0)
		}
	}

	return nil
}

func (s *Supervisor) Sink() error {
	mode := os.Getenv(ENV_SUPERVISOR)

	switch mode {
	case "":
		if err := s.Daemon.Sink(); err != nil {
			return err
		}

		// as a foreground process, but give daemon a chance to
		// setup signal/pid related things
		if s.Foreground {
			return nil
		}

		// we should be session leader here
		if err := os.Setenv(ENV_SUPERVISOR, "worker"); err != nil {
			fatal(err)
		}
		s.supervise()
		log.Fatal("BUG, supervisor should loop forever") //should never get here
	case "worker":
		if err := unsetenv(ENV_SUPERVISOR); err != nil {
			fatal(err)
		}

	default:
		err := fmt.Errorf("critical error, unknown mode: %s", mode)
		fmt.Println(err)
		log.Println(err)
		os.Exit(1)
	}

	return nil
}

// Handle will install a handler to supervisor, just one handler can be added
func (s *Supervisor) Handle(h Handler) error {
	if len(s.cmds) > 1 {
		return errors.New("Handler already existed")
	}

	s.Daemon.Handle(h)
	c := &s.Daemon.Cmd
	cmd := &Cmd{Cmd: c}
	s.cmds = append(s.cmds, cmd)

	return nil
}

func (s *Supervisor) HandleFunc(f func() error) error {
	h := HandlerFunc(f)
	return s.Handle(h)
}

func (s *Supervisor) AddCommand(name string, arg ...string) error {
	if name == "" {
		return errors.New("Empty Command")
	}

	cmd := NewCmd(name, arg...)
	s.cmds = append(s.cmds, cmd)

	return nil
}

func Handle(pidfile string, foreground bool, h Handler) SinkServer {
	DefaultSupervisor.PidFile = pidfile
	DefaultSupervisor.Foreground = foreground
	DefaultSupervisor.Handle(h)
	return DefaultSupervisor
}

func HandleFunc(pidfile string, foreground bool, f func() error) SinkServer {
	DefaultSupervisor.PidFile = pidfile
	DefaultSupervisor.Foreground = foreground
	DefaultSupervisor.HandleFunc(f)
	return DefaultSupervisor
}

func AddCommand(foreground bool, name string, arg ...string) (SinkServer, error) {
	if err := DefaultSupervisor.AddCommand(name, arg...); err != nil {
		return nil, err
	} else {
		return DefaultSupervisor, nil
	}
}
