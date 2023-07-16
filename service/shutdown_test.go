package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/birdie-ai/golibs/service"
)

func TestShutdown(t *testing.T) {
	handler := service.NewShutdownHandler(time.Minute)
	service1 := newFakeService()
	service2 := newFakeService()

	handler.Add(service1)
	handler.Add(service2)

	ctx, cancel := context.WithCancel(context.Background())
	waitErr := make(chan error)
	go func() {
		waitErr <- handler.Wait(ctx)
		close(waitErr)
	}()

	// Guarantee that shutdown is not called before cancellation
	// Not actual guarantee, but should catch stupid bugs
	select {
	case <-service1.calls:
		t.Fatal("service 1 shutdown called")
	case <-service2.calls:
		t.Fatal("service 1 shutdown called")
	case <-time.NewTimer(50 * time.Millisecond).C:
		break
	}

	cancel()
	// Guarantee that shutdown is called for all services concurrently
	// We first read both calls before sending any answer
	service1Call := <-service1.calls
	service2Call := <-service2.calls

	checkShutdownHandlerIsWaiting := func() {
		select {
		case err := <-waitErr:
			t.Fatalf("handler.Wait() returned before services shutting down: %v", err)
		case <-time.NewTimer(50 * time.Millisecond).C:
			break
		}
	}

	// Guarantee that the shutdown handler only stops waiting when ALL services are done.

	checkShutdownHandlerIsWaiting()
	service1Call.sendResponse(nil)

	checkShutdownHandlerIsWaiting()
	service2Call.sendResponse(nil)

	if err := <-waitErr; err != nil {
		t.Fatal(err)
	}
}

func TestShutdownErrorAggregation(t *testing.T) {
	handler := service.NewShutdownHandler(time.Minute)
	service1 := newFakeService()
	service2 := newFakeService()
	service3 := newFakeService()

	handler.Add(service1)
	handler.Add(service2)
	handler.Add(service3)

	ctx, cancel := context.WithCancel(context.Background())
	waitErr := make(chan error)
	go func() {
		waitErr <- handler.Wait(ctx)
		close(waitErr)
	}()
	cancel()
	// Guarantee that shutdown is called for all services concurrently
	// We first read both calls before sending any answer
	service1Call := <-service1.calls
	service2Call := <-service2.calls
	service3Call := <-service3.calls

	err1 := errors.New("service 1 error")
	service1Call.sendResponse(err1)

	service2Call.sendResponse(nil)

	err3 := errors.New("service 3 error")
	service3Call.sendResponse(err3)

	err := <-waitErr
	if err == nil {
		t.Fatal("want error, got nil")
	}

	if !strings.Contains(err.Error(), err1.Error()) {
		t.Fatalf("error %v does not contain service 1 error %v", err, err1)
	}
	if !strings.Contains(err.Error(), err3.Error()) {
		t.Fatalf("error %v does not contain service 1 error %v", err, err3)
	}
}

type (
	shutdownCall struct {
		response chan error
	}
	fakeService struct {
		calls chan shutdownCall
	}
)

func newFakeService() *fakeService {
	return &fakeService{
		calls: make(chan shutdownCall),
	}
}

func (f *fakeService) Shutdown(context.Context) error {
	call := shutdownCall{
		response: make(chan error),
	}
	f.calls <- call
	return <-call.response
}

func (s *shutdownCall) sendResponse(err error) {
	s.response <- err
	close(s.response)
}
