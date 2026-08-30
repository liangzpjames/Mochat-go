package runtimegroup

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
)

// Drainer removes the process from readiness before listeners stop accepting work.
type Drainer interface {
	BeginDrain()
}

type service struct {
	name     string
	listener net.Listener
	server   *http.Server
}

// Group owns all HTTP listeners and drains them as one process service group.
type Group struct {
	drainer         Drainer
	stopNewWork     func()
	shutdownTimeout time.Duration
	services        []service
	drainOnce       sync.Once
}

func New(drainer Drainer, stopNewWork func(), shutdownTimeout time.Duration) *Group {
	if shutdownTimeout <= 0 {
		shutdownTimeout = 30 * time.Second
	}
	return &Group{drainer: drainer, stopNewWork: stopNewWork, shutdownTimeout: shutdownTimeout}
}

// NewHTTPServer applies the process-wide HTTP bounds. The request base context
// retains root values but deliberately does not inherit root cancellation: an
// in-flight request is owned by http.Server.Shutdown during drain.
func NewHTTPServer(root context.Context, handler http.Handler) *http.Server {
	if root == nil {
		root = context.Background()
	}
	if handler == nil {
		handler = http.NotFoundHandler()
	}
	requestBase := context.WithoutCancel(root)
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		BaseContext: func(net.Listener) context.Context {
			return requestBase
		},
	}
}

func (g *Group) Add(name string, listener net.Listener, server *http.Server) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("http service name is required")
	}
	if listener == nil {
		return fmt.Errorf("http service %s listener is required", name)
	}
	if server == nil {
		return fmt.Errorf("http service %s server is required", name)
	}
	for _, existing := range g.services {
		if existing.name == name {
			return fmt.Errorf("duplicate http service name: %s", name)
		}
	}
	g.services = append(g.services, service{name: name, listener: listener, server: server})
	return nil
}

func (g *Group) Run(shutdown context.Context) error {
	if shutdown == nil {
		shutdown = context.Background()
	}
	if len(g.services) == 0 {
		<-shutdown.Done()
		g.beginDrain()
		return nil
	}

	serveErrors := make(chan error, len(g.services))
	for _, current := range g.services {
		current := current
		go func() {
			err := current.server.Serve(current.listener)
			if err != nil {
				serveErrors <- fmt.Errorf("serve %s: %w", current.name, err)
				return
			}
			serveErrors <- fmt.Errorf("serve %s stopped unexpectedly", current.name)
		}()
	}

	var runErr error
	select {
	case <-shutdown.Done():
	case runErr = <-serveErrors:
	}
	g.beginDrain()

	drainCtx, cancel := context.WithTimeout(context.Background(), g.shutdownTimeout)
	defer cancel()
	var wait sync.WaitGroup
	shutdownErrors := make(chan error, len(g.services))
	for _, current := range g.services {
		current := current
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := current.server.Shutdown(drainCtx); err != nil {
				_ = current.server.Close()
				shutdownErrors <- fmt.Errorf("shutdown %s: %w", current.name, err)
			}
		}()
	}
	wait.Wait()
	close(shutdownErrors)
	if runErr != nil {
		return runErr
	}
	for err := range shutdownErrors {
		return err
	}
	return nil
}

func (g *Group) beginDrain() {
	g.drainOnce.Do(func() {
		if g.drainer != nil {
			g.drainer.BeginDrain()
		}
		if g.stopNewWork != nil {
			g.stopNewWork()
		}
	})
}
