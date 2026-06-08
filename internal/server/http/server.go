package http

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Server struct {
	httpServer *http.Server
	cleanup    func() error
}

func New(addr string, handler http.Handler, cleanup func() error) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		cleanup: cleanup,
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
			return err
		}
		return s.cleanupResources()
	}
}

func (s *Server) stop(reason error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return err
	}
	if err := s.cleanupResources(); err != nil {
		return err
	}

	return reason
}

func (s *Server) cleanupResources() error {
	if s.cleanup == nil {
		return nil
	}
	return s.cleanup()
}
