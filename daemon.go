// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package nestor

import (
	"errors"
	"fmt"
	"github.com/zhgwenming/gbalancer/utils"
	stdlog "log"
	"log/syslog"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"
)

const (
	ENV_DAEMON = "__GO_DAEMON_MODE"
)

var (
	DefaultDaemon = NewDaemon()
	log           = NewLogger()
)

type Handler interface {
	Serve() error
	Stop() error
}

type Daemon struct {
	PidFile     string
	Foreground  bool
	Signalc     chan os.Signal
	Cmd         exec.Cmd
	WaitSeconds time.Duration
	Log         logFile
	h           Handler
}

func NewDaemon() *Daemon {
	d := &Daemon{WaitSeconds: time.Second}
	d.Signalc = make(chan os.Signal, 1)

	return d
}

func NewLogger() (l *stdlog.Logger) {
	// try to use syslog first
	if logger, err := syslog.NewLogger(syslog.LOG_NOTICE, 0); err != nil {
		l = stdlog.New(os.Stderr, "", stdlog.LstdFlags)
	} else {
		l = logger
	}
	return
}

func fatal(err error) {
	log.Printf("error: %s\n", err)
	os.Exit(1)
}

func (d *Daemon) setupPidfile() {
	if d.PidFile == "" {
		return
	}

	if err := utils.WritePid(d.PidFile); err != nil {
		log.Printf("error: %s\n", err)
		os.Exit(1)
	}
}

func (d *Daemon) cleanPidfile() {
	if d.PidFile == "" {
		return
	}

	if err := os.Remove(d.PidFile); err != nil {
		log.Printf("error to remove pidfile %s:", err)
	}
}

func (d *Daemon) createLogfile() (*os.File, error) {
	var err error

	if d.Log.path == "" {
		logfile := "/tmp/" + path.Base(os.Args[0]) + ".log"
		d.Log.path = logfile
	}

	if err = d.Log.Open(); err != nil {
		fmt.Printf("- Failed to create output log file - %s: %s\n", d.Log.path, err)
	}

	if err != nil {
		return nil, err
	} else {
		return d.Log.file, nil
	}
}

// monitor or the worker process
func (d *Daemon) child() {
	os.Chdir("/")

	// Setsid in the exec.Cmd.SysProcAttr.Setsid
	//syscall.Setsid()

	d.setupPidfile()
}

func (d *Daemon) parent() {
	signal.Notify(d.Signalc,
		syscall.SIGCHLD)

	cmd := d.Cmd

	procAttr := &syscall.SysProcAttr{Setsid: true}
	cmd.SysProcAttr = procAttr

	if file, err := d.createLogfile(); err == nil {
		fmt.Printf("- redirected the output to %s\n", file.Name())
		cmd.Stdout = file
		cmd.Stderr = file
	}

	if err := cmd.Start(); err == nil {
		fmt.Printf("- Started daemon as pid %d\n", cmd.Process.Pid)
		select {
		case <-time.After(time.Second / 5):
		case sig := <-d.Signalc:
			if sig == syscall.SIGCHLD {
				if err := cmd.Wait(); err != nil {
					fmt.Printf("- daemon exited with %s\n", err)
					d.Log.Dump(os.Stderr)
				}
			}
		}
		os.Exit(0)
	} else {
		fmt.Printf("error to run in daemon mode - %s\n", err)
		os.Exit(1)
	}
}

// RunWait will run the specified function in safe mode, it blocks the caller until it finished
func (d *Daemon) RunWait(handler func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered f %s\nbacktrace:\n%s", r, debug.Stack())
			switch x := r.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("Unknown panic")
			}
			err = newError(ErrPanic)
		}
	}()

	err = handler()
	return err
}

func (d *Daemon) runLoop(handler func() error) {
	for {
		startTime := time.Now()

		// exit goroutine if handler finished w/o errors
		if err := d.RunWait(handler); err == nil {
			return
		}

		for {
			endTime := time.Now()
			duration := endTime.Sub(startTime)
			if duration.Seconds() > 5 {
				break
			} else {
				time.Sleep(time.Second)
			}
		}
	}
}

// RunForever returns imediately to the caller and run the specified function
// in background, it watches over the requested function in a separate
// goroutine, the function will get restarted infinitely on errors.
func (d *Daemon) RunForever(handler func() error) {
	go d.runLoop(handler)
}

// RunForever returns imediately to the caller and run the specified function in background
func (d *Daemon) RunOnce(handler func() error) {
	go d.RunWait(handler)
}

// selfCmd run before Sink
func (d *Daemon) setSelfCmd() error {
	// make path as abs path or relative path
	cmdPath, err := exec.LookPath(os.Args[0])
	if err != nil {
		return err
	}

	if p, err := filepath.Abs(cmdPath); err != nil {
		return err
	} else {
		d.Cmd = exec.Cmd{
			Path: p,
			Args: os.Args,
		}
	}

	return nil
}

// Start will setup the daemon environment and create pidfile if pidfile is not empty
// Parent process will never return
// Will return back to the worker process
func (d *Daemon) Sink() error {
	if d.Cmd.Path == "" {
		if d.h != nil {
			if err := d.setSelfCmd(); err != nil {
				fatal(err)
			}
		} else {
			return fmt.Errorf("Handler or Command should be specified first")
		}
	} else {
		if d.h != nil {
			return fmt.Errorf("Handler couldn't coexist with Command")
		}
	}

	// the signal handler is needed for both parent and child
	// since we need to support foreground mode
	signal.Notify(d.Signalc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGTERM)

	if d.PidFile != "" {
		if _, err := os.Stat(path.Dir(d.PidFile)); os.IsNotExist(err) {
			return err
		}

		// switch to use abs pidfile, background daemon will chdir to /
		if p, err := filepath.Abs(d.PidFile); err != nil {
			fatal(err)
		} else {
			d.PidFile = p
		}
	}

	// as a foreground process
	if d.Foreground {
		fmt.Printf("- Running as foreground process\n")
		d.setupPidfile()
		return nil
	}

	// parent/child/worker logic
	// background monitor/worker process, all the magic goes here
	mode := os.Getenv(ENV_DAEMON)

	switch mode {
	case "":
		err := os.Setenv(ENV_DAEMON, "child")
		if err != nil {
			fatal(err)
		}

		d.parent()                           // fork and exit
		log.Fatal("BUG, parent didn't exit") //should never get here
	case "child":
		if err := unsetenv(ENV_DAEMON); err != nil {
			fatal(err)
		}

		d.child()
	default:
		err := fmt.Errorf("critical error, unknown mode: %s", mode)
		fmt.Println(err)
		log.Println(err)
		os.Exit(1)
	}

	return nil
}

func (d *Daemon) Serve() {
	// handler serve
	d.h.Serve()
}

func (d *Daemon) WaitSignal() {
	// waiting for exit signals
	for sig := range d.Signalc {
		log.Printf("captured %v, exiting..\n", sig)
		// exit if we get any signal
		// Todo - catch signal other than SIGTERM/SIGINT
		break
	}

	// handler stop routine
	d.h.Stop()

	d.cleanPidfile()
	return
}

func (d *Daemon) Command(name string, arg ...string) error {
	d.Cmd = exec.Cmd{
		Path: name,
		Args: append([]string{name}, arg...),
	}
	if filepath.Base(name) == name {
		if lp, err := exec.LookPath(name); err != nil {
			return err
		} else {
			d.Cmd.Path = lp
		}
	}

	return nil
}

type HandlerFunc func() error

func (h HandlerFunc) Serve() error {
	return h()
}

func (h HandlerFunc) Stop() error {
	return nil
}

func (d *Daemon) Handle(h Handler) {
	d.h = h
}

func (d *Daemon) HandleFunc(f func() error) {
	h := HandlerFunc(f)
	d.Handle(h)
}

type SinkServer interface {
	Sink() error
	Serve()
	WaitSignal()
}

func DaemonHandle(pidfile string, foreground bool, h Handler) SinkServer {
	DefaultDaemon.PidFile = pidfile
	DefaultDaemon.Foreground = foreground
	DefaultDaemon.Handle(h)
	return DefaultDaemon
}

func DaemonHandleFunc(pidfile string, foreground bool, f func() error) SinkServer {
	DefaultDaemon.PidFile = pidfile
	DefaultDaemon.Foreground = foreground
	DefaultDaemon.HandleFunc(f)
	return DefaultDaemon
}

func DaemonCommand(pidfile string, name string, arg ...string) (SinkServer, error) {
	if err := DefaultDaemon.Command(name, arg...); err != nil {
		return nil, err
	} else {
		return DefaultDaemon, nil
	}
}

// a function calls different sink functions
func Start(s SinkServer) error {

	if err := s.Sink(); err != nil {
		return err
	}

	// handler serve
	s.Serve()

	// wait to exit
	s.WaitSignal()
	return nil
}
