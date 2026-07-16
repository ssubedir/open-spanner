package http

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Server struct {
	httpServer  *http.Server
	beginDrain  func()
	cleanup     func() error
	beginOnce   sync.Once
	cleanupOnce sync.Once
	cleanupErr  error
}

func New(addr string, handler http.Handler, cleanup func() error, beginDrain ...func()) *Server {
	var begin func()
	if len(beginDrain) > 0 {
		begin = beginDrain[0]
	}
	return &Server{
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		beginDrain: begin,
		cleanup:    cleanup,
	}
}

func (s *Server) Run(ctx context.Context) error {
	errs := make(chan error, 1)

	go func() {
		log.Printf("listening on %s", s.httpServer.Addr)
		err := s.httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
			return
		}
		errs <- nil
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(shutdown)

	select {
	case <-ctx.Done():
		return s.stop(ctx.Err())
	case signal := <-shutdown:
		log.Printf("shutdown requested: %s", signal)
		return s.stop(nil)
	case err := <-errs:
		if err != nil {
			return errors.Join(err, s.stop(nil))
		}
		return s.stop(nil)
	}
}

func (s *Server) stop(reason error) error {
	s.beginOnce.Do(func() {
		if s.beginDrain != nil {
			s.beginDrain()
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	shutdownErr := s.httpServer.Shutdown(ctx)
	var forceErr error
	if shutdownErr != nil {
		forceErr = s.httpServer.Close()
	}
	cleanupErr := s.cleanupResources()
	return errors.Join(reason, shutdownErr, forceErr, cleanupErr)
}

func (s *Server) cleanupResources() error {
	s.cleanupOnce.Do(func() {
		if s.cleanup != nil {
			s.cleanupErr = s.cleanup()
		}
	})
	return s.cleanupErr
}
