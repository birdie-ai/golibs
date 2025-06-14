// Package service provides functionality common to services
package service

import (
	"context"
	"errors"
	"time"

	"golang.org/x/sync/errgroup"
)

// Shutdowner represents a service that can shutdown.
type Shutdowner interface {
	Shutdown(context.Context) error
}

// ShutdownHandler handles the shutdown of multiple services.
// It waits for a context to be cancelled to then call each service's Shutdown method.
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
// When [ctx] is cancelled it will shut down all services
// concurrently and wait for all of them to finish before returning.
// It will wait for each service to shut down for the wait period provided on
// NewShutdownHandler.
func (s *ShutdownHandler) Wait(ctx context.Context) error {
	<-ctx.Done()

	errs := make([]error, len(s.services))
	g := &errgroup.Group{}

	for i, service := range s.services {
		g.Go(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), s.waitPeriod)
			defer cancel()
			errs[i] = service.Shutdown(ctx)
			return nil
		})
	}
	// We never fail the task, we accumulate all errors on errs.
	_ = g.Wait()

	return errors.Join(errs...)
}
