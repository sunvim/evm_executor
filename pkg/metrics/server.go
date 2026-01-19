package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/sunvim/evm_executor/pkg/logger"
)

// Server is the Prometheus metrics server
type Server struct {
	addr   string
	server *http.Server
}

// NewServer creates a new metrics server
func NewServer(addr string) *Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	return &Server{
		addr: addr,
		server: &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	}
}

// Start starts the metrics server
func (s *Server) Start() error {
	logger.Infof("Starting metrics server on %s", s.addr)
	
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("Metrics server error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully stops the metrics server
func (s *Server) Stop() error {
	logger.Info("Stopping metrics server")
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown metrics server: %w", err)
	}

	logger.Info("Metrics server stopped")
	return nil
}
