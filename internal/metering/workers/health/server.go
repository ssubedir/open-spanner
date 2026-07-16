package health

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

type ReadyChecker interface {
	Ready(context.Context) error
}

type Logger func(string, ...any)

type Server struct {
	server   *http.Server
	listener net.Listener
	checker  ReadyChecker
	logger   Logger
	draining atomic.Bool
}

func Start(addr string, checker ReadyChecker, logger Logger) (*Server, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	server := &Server{listener: listener, checker: checker, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", server.health)
	mux.HandleFunc("GET /ready", server.ready)
	server.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	go func() {
		if err := server.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) && logger != nil {
			logger("worker health server failed: %v", err)
		}
	}()
	return server, nil
}

func (s *Server) Addr() net.Addr { return s.listener.Addr() }

func (s *Server) BeginDrain() { s.draining.Store(true) }

func (s *Server) Shutdown(ctx context.Context) error {
	s.BeginDrain()
	return s.server.Shutdown(ctx)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if s.draining.Load() || s.checker == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.checker.Ready(ctx); err != nil {
		if s.logger != nil {
			s.logger("worker readiness check failed: %v", err)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
