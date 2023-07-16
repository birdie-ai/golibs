// Package service provides functionality common to services
package service

import (
	"context"
	"sync"
	"time"

	"github.com/birdie-ai/golibs/slog"
)

// Shutdowner represents a service that can shutdown.
type Shutdowner interface {
	Shutdown(context.Context) error
}

// ShutdownHandler handles the shutdown of multiple services.
// It waits for a context to be cancelled to then call all added services Shutdown methods.
type ShutdownHandler struct {
	waitPeriod time.Duration
	services   []Shutdowner
}

// NewShutdownHandler creates a new [ShutdownHandler] with the given [gracefulShutdownPeriod].
func NewShutdownHandler(gracefulShutdownPeriod time.Duration) *ShutdownHandler {
	return &ShutdownHandler{waitPeriod: gracefulShutdownPeriod}
}

// Add will add the given service to the handler.
// Must be called before [ShutdownHandler.Wait] is called.
func (s *ShutdownHandler) Add(service Shutdowner) {
	s.services = append(s.services, service)
}

// Wait will wait for the given [ctx] to be cancelled.
// When [ctx] is cancelled it will shutdown all services
// concurrently and wait for all of them to finish before returning.
// It will wait for each service to shutdown for the wait period provided on
// NewShutdownHandler.
func (s *ShutdownHandler) Wait(ctx context.Context) {
	<-ctx.Done()

	log := slog.FromCtx(ctx)
	log.Debug("received shutdown signal, shutting down all services")

	wg := &sync.WaitGroup{}
	wg.Add(len(s.services))

	for _, v := range s.services {
		service := v

		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), s.waitPeriod)
			defer cancel()

			if err := service.Shutdown(ctx); err != nil {
				log.Error("failed to shutdown service", "error", err)
			}
		}()
	}

	wg.Wait()

	log.Debug("finished shutting down all services")
}
