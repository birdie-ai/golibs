package xhttp_test

import (
	"fmt"
	"net/http"
	"sync"
)

// FakeClient allows to fake interactions with an [xhttp.Client].
type FakeClient struct {
	requests  []*http.Request
	responses []response
	mutex     sync.Mutex
}

func NewFakeClient() *FakeClient {
	return &FakeClient{}
}

// PushResponse will push the given response on the response queue of this [FakeClient].
// Calls to [FakeClient.Do] will use the provided responses and will give an error when no
// response is defined for a request. Pushed responses are handled in a FIFO manner (queue).
func (f *FakeClient) PushResponse(res *http.Response) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	f.responses = append(f.responses, response{
		res: res,
	})
}

// PushError will push the given error on the response queue of this [FakeClient].
// Calls to [FakeClient.Do] will use the provided error as a result to a request.
// Errors are enqueued with success responses [FakeClient.PushResponse].
func (f *FakeClient) PushError(err error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	f.responses = append(f.responses, response{
		err: err,
	})
}

// Requests returns all received requests on this client.
func (f *FakeClient) Requests() []*http.Request {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	return f.requests
}

func (f *FakeClient) Do(req *http.Request) (*http.Response, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.responses) == 0 {
		return nil, fmt.Errorf("no response configured on FakeClient for request: %v", req)
	}

	f.requests = append(f.requests, req)

	response := f.responses[0]
	f.responses = f.responses[1:]
	return response.res, response.err
}

type response struct {
	res *http.Response
	err error
}
