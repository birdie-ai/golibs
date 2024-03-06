package xhttptest

import (
	"fmt"
	"io"
	"net/http"
	"sync"
)

// Client allows to fake interactions with an [xhttp.Client].
// It is safe to use the client concurrently.
type Client struct {
	requests  []*http.Request
	responses []response
	mutex     sync.Mutex
	callback  func(*http.Request)
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

	// The real http.Do guarantees that responses always have a non nil body, lets do the same
	if res.Body == nil {
		res.Body = &nopReaderCloser{}
	}
	c.responses = append(c.responses, response{
		res: res,
	})
}

// OnDo defines a callback that is called for each Do call on this fake client.
// It doesn't allow to inject responses, it is designed only to observe requests
// or do something between a request and the response is returned to the caller.
// If the callback blocks it will block all other calls to Do until the callback returns,
// callback calls are serial even if Do is called concurrently.
func (c *Client) OnDo(callback func(*http.Request)) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.callback = callback
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

	return c.requests
}

// Do records requests and sends responses/errors.
// To control responses/error use [Client.PushResponse] and [Client.PushError].
// To check received requests use [Client.Requests].
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.callback != nil {
		c.callback(req)
	}

	// We need to clone the request since the original request may be mutated after this method returns
	c.requests = append(c.requests, req.Clone(req.Context()))

	if len(c.responses) == 0 {
		return nil, fmt.Errorf("no response configured on FakeClient for request: %v", req)
	}

	response := c.responses[0]
	c.responses = c.responses[1:]
	return response.res, response.err
}

type (
	response struct {
		res *http.Response
		err error
	}
	nopReaderCloser struct{}
)

func (*nopReaderCloser) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (*nopReaderCloser) Close() error {
	return nil
}
