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

var (
	servers = make(map[Server]net.Listener, 0)
)

type Server interface {
	Listener() (net.Listener, error)
	Startup(ln net.Listener)
	Shutdown(ctx context.Context)
}

func Startup(servers ...Server) error {
	for _, s := range servers {
		ln, err := ensureListener(s)
		if nil != err {
			return err
		}

		startup(s, ln)
	}
	return nil
}

func startup(s Server, ln net.Listener) {
	servers[s], _ = s.Listener()
	go s.Startup(ln)
}

func Wait() error {
	sign := make(chan os.Signal)
	signal.Notify(sign, syscall.SIGUSR2)

	for {
		select {
		case sig := <-sign:
			switch sig {
			case syscall.SIGUSR2:
				if err := restart(); err != nil {
					return err
				}
			}
		}
	}
}

func fork() error {
	files := []uintptr{
		os.Stdin.Fd(),
		os.Stdout.Fd(),
		os.Stdout.Fd(),
	}
	for _, ln := range servers {
		if ln != nil {
			lnFile, err := listenerFile(ln)
			if err != nil {
				return err
			}
			files = append(files, lnFile.Fd())

			if err := os.Setenv(`_LISTENER_`+ln.Addr().String(), lnFile.Name()); err != nil {
				return err
			}
		}
	}

	env := os.Environ()
	exec, err := os.Executable()

	if nil != err {
		return err
	}

	dir := filepath.Dir(exec)

	_, err = syscall.ForkExec(os.Args[0], os.Args, &syscall.ProcAttr{
		Dir:   dir,
		Env:   env,
		Files: files,
	})

	return err
}

func listenerFile(ln net.Listener) (*os.File, error) {
	if nil == ln {
		return nil, nil
	}

	switch t := ln.(type) {
	case *net.TCPListener:
		return t.File()
	case *net.UnixListener:
		return t.File()
	default:
		return nil, errors.New(`unsupported listener`)
	}
}

func restart() error {
	if err := fork(); err != nil {
		return err
	}

	if p, err := os.FindProcess(os.Getpid()); err != nil {
		return err
	} else {
		return p.Kill()
	}
}

func ensureListener(s Server) (net.Listener, error) {
	ln, has := servers[s]

	if has {
		env := os.Getenv(`_LISTENER_` + ln.Addr().String())

		if env != `` {
			lnFile := os.NewFile(3, env)
			if lnFile == nil {
				return nil, errors.New(`unable to created listener file ` + env)
			}
			defer lnFile.Close()

			return net.FileListener(lnFile)
		}
	}

	return s.Listener()
}
