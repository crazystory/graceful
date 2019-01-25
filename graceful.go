package graceful

import (
	"context"
	"errors"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

type Server interface {
	Listener() (net.Listener, error)
	Startup(ln net.Listener)
	Shutdown(ctx context.Context)
}

func Startup(s Server) error {
	ln, err := ensureListener(s)
	if nil != err {
		return err
	}

	startup(s, ln)

	return wait(s, ln)
}

func startup(s Server, ln net.Listener) {
	go s.Startup(ln)
}

func wait(s Server, ln net.Listener) error {
	sign := make(chan os.Signal)
	signal.Notify(sign, syscall.SIGUSR2)

	for {
		select {
		case sig := <-sign:
			switch sig {
			case syscall.SIGUSR2:
				if err := restart(s, ln); err != nil {
					return err
				}
			}
		}
	}
}

func fork(ln net.Listener) (*os.Process, error) {
	lnFile, err := listenerFile(ln)

	if nil != err {
		return nil, err
	}

	defer lnFile.Close()
	files := []*os.File{
		os.Stdin,
		os.Stdout,
		os.Stderr,
		lnFile,
	}

	exec, err := os.Executable()

	if nil != err {
		return nil, err
	}

	dir := filepath.Dir(exec)
	env := append(os.Environ(), `LISTENER_FILE=`+lnFile.Name())

	process, err := os.StartProcess(exec, []string{exec}, &os.ProcAttr{
		Dir:   dir,
		Env:   env,
		Files: files,
		Sys:   &syscall.SysProcAttr{},
	})
	if nil != err {
		return nil, err
	}

	return process, nil
}

func listenerFile(ln net.Listener) (*os.File, error) {
	switch t := ln.(type) {
	case *net.TCPListener:
		return t.File()
	case *net.UnixListener:
		return t.File()
	}
	return nil, errors.New(`unsupported listener`)
}

func restart(s Server, ln net.Listener) error {
	if _, err := fork(ln); err != nil {
		return err
	}

	return syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

func ensureListener(s Server) (net.Listener, error) {
	env := os.Getenv(`LISTENER_FILE`)

	if env != `` {
		lnFile := os.NewFile(3, env)
		if lnFile == nil {
			return nil, errors.New(`unable to created listener file ` + env)
		}
		defer lnFile.Close()

		return net.FileListener(lnFile)
	}

	return s.Listener()
}
