package service

import (
	"testing"
	"context"
)

func TestGracefulShutdown(t *testing.T) {
}

type (
	shutdownCall struct {
		response chan error
	}
	fakeService struct{
		calls chan shutdownCall
	}
)

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
