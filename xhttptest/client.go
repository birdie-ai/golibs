package xhttptest

import (
	"fmt"
	"net/http"
	"sync"
)

// Client allows to fake interactions with an [xhttp.Client].
// It is safe to use the client concurrently.
type Client struct {
	requests  []*http.Request
	responses []response
	mutex     sync.Mutex
}

// NewClient creates a http client for test purposes.
func NewClient() *Client {
	return &Client{}
}

// PushResponse will push the given response on the response queue of this [FakeClient].
// Calls to [FakeClient.Do] will use the provided responses and will give an error when no
// response is defined for a request. Pushed responses are handled in a FIFO manner (queue).
func (c *Client) PushResponse(res *http.Response) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.responses = append(c.responses, response{
		res: res,
	})
}

// PushError will push the given error on the response queue of this [FakeClient].
// Calls to [FakeClient.Do] will use the provided error as a result to a request.
// Errors are enqueued with success responses [FakeClient.PushResponse].
func (c *Client) PushError(err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.responses = append(c.responses, response{
		err: err,
	})
}

// Requests returns all received requests on this client.
// It returns cloned requests, so the caller is guaranteed to not see any changes
// on the underlying requests.
func (c *Client) Requests() []*http.Request {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	clonedReqs := make([]*http.Request, len(c.requests))
	for i, req := range c.requests {
		clonedReqs[i] = req.Clone(req.Context())
	}
	return clonedReqs
}

// Do records requests and sends responses/errors.
// To control responses/error use [Client.PushResponse] and [Client.PushError].
// To check received requests use [Client.Requests].
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.requests = append(c.requests, req)

	if len(c.responses) == 0 {
		return nil, fmt.Errorf("no response configured on FakeClient for request: %v", req)
	}

	response := c.responses[0]
	c.responses = c.responses[1:]
	return response.res, response.err
}

type response struct {
	res *http.Response
	err error
}
