package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/simplejia/clog"
)

var GRACEFUL_ENV = "GRACEFUL=true"

type Server struct {
	server   *http.Server
	listener net.Listener

	isGraceful bool
	waitTime   time.Duration
	signalChan chan os.Signal
}

func NewServer(addr string, handler http.Handler) *Server {
	isGraceful := false

	GRACEFUL_ENV = filepath.Base(os.Args[0]) + "_" + GRACEFUL_ENV

	for _, v := range os.Environ() {
		if v == GRACEFUL_ENV {
			isGraceful = true
			break
		}
	}

	return &Server{
		server: &http.Server{
			Addr:    addr,
			Handler: handler,
		},

		isGraceful: isGraceful,
		signalChan: make(chan os.Signal),
		waitTime:   time.Second * 5, // 暂定
	}
}

func (srv *Server) ListenAndServe() (err error) {
	var ln net.Listener

	if srv.isGraceful {
		file := os.NewFile(3, "")
		ln, err = net.FileListener(file)
		if err != nil {
			return
		}
	} else {
		ln, err = net.Listen("tcp", srv.server.Addr)
		if err != nil {
			return
		}
	}

	srv.listener = ln

	go func() {
		if err := srv.server.Serve(srv.listener); err != http.ErrServerClosed {
			clog.Error("http Serve error: %v", err)
		}
	}()

	srv.handleSignal()

	return
}

func (srv *Server) handleSignal() {
	signal.Notify(srv.signalChan, syscall.SIGHUP)
	for {
		<-srv.signalChan

		err := srv.fork()
		if err != nil {
			clog.Error("start new process failed, please retry: %v", err)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), srv.waitTime)
		srv.server.Shutdown(ctx)
		cancel()
		break
	}
}

func (srv *Server) fork() (err error) {
	clog.Info("grace restart...")

	var env []string
	for _, v := range os.Environ() {
		if v != GRACEFUL_ENV {
			env = append(env, v)
		}
	}
	env = append(env, GRACEFUL_ENV)

	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	file, _ := srv.listener.(*net.TCPListener).File()
	cmd.ExtraFiles = []*os.File{file}

	err = cmd.Start()
	if err != nil {
		return
	}

	return
}

func ListenAndServe(addr string, handler http.Handler) error {
	return NewServer(addr, handler).ListenAndServe()
}
