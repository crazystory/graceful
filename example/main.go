package main

import (
	"context"
	"fmt"
	"github.com/crazystory/graceful"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

type server struct {
	server http.Server
	message string
}

func (s *server) Startup(ln net.Listener) error {
	pidPath := `example.pid`
	currentPid := os.Getpid()
	if err := os.MkdirAll(filepath.Dir(pidPath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create PID folder: %v", err)
	}

	file, err := os.Create(pidPath)
	if err != nil {
		return fmt.Errorf("failed to create PID file: %v", err)
	}
	defer file.Close()
	if _, err := file.WriteString(strconv.FormatInt(int64(currentPid), 10)); err != nil {
		return fmt.Errorf("failed to write PID information: %v", err)
	}

	http.HandleFunc(`/`, func(writer http.ResponseWriter, request *http.Request) {
		fmt.Fprint(writer, s.message)
	})

	return s.server.Serve(ln)
}

func (s *server) Listener() (net.Listener, error) {
	return net.Listen(`tcp`, `:8080`)
}

func (s *server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == `restart` {

		content, err := ioutil.ReadFile(`example.pid`)
		if err != nil {
			log.Fatal(err.Error())
		}

		pid, _ := strconv.Atoi(string(content))

		if err := syscall.Kill(pid, syscall.SIGUSR2); err != nil {
			log.Fatal(err.Error())
		}
	} else {
		/**
		 * curl 127.0.0.1:8080 确认获取到hello
		 * 修改 &server{message: `hello`, server: http.Server{}} 为 &server{message: `world`, server: http.Server{}}
		 * 执行 go build && ./example restart
		 * curl 127.0.0.1:8080 确认获取到world
		 */
		if err := graceful.Startup(&server{message: `hello`, server: http.Server{}}); err != nil {
			log.Fatal(err.Error())
		}

		if err := graceful.Wait(); err != nil {
			log.Fatalf(err.Error())
		}
	}
}
