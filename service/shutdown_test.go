package service_test

import (
	"context"
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
	waitDone := make(chan struct{})
	go func() {
		handler.Wait(ctx)
		close(waitDone)
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
		case <-waitDone:
			t.Fatal("handler.Wait() returned before services shutting down")
		case <-time.NewTimer(50 * time.Millisecond).C:
			break
		}
	}

	// Guarantee that the shutdown handler only stops waiting when ALL services are done.

	checkShutdownHandlerIsWaiting()
	service1Call.sendResponse(nil)

	checkShutdownHandlerIsWaiting()
	service2Call.sendResponse(nil)

	<-waitDone
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
