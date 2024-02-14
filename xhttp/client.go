package xhttp

import "net/http"

type (
	// Client abstracts a [http.Client], allowing us to create wrappers for http clients adding useful
	// functionality like retry and metrics. It has the same API as [http.Client] and is intended to be
	// a drop-in replacement (but not all methods are supported yet).
	Client interface {
		Do(req *http.Request) (*http.Response, error)
	}
	// RetrierOption is used to configure retrier clients created with [NewRetrierClient].
	RetrierOption func(*retrierClient)
)

// RetrierWithRequestTimeout configures a client retrier with the given timeout. This timeout is used per request/try.
// When calling [Client.Do] if the request has a context with a deadline longer than this timeout the retrier
// will keep retrying until the request context is cancelled/deadline expires.
func RetrierWithRequestTimeout(int) RetrierOption {
	return func(*retrierClient) {
		// TODO
	}
}

// NewRetrierClient wraps the given client with retry logic.
// The returned [Client] will automatically retry failed requests.
func NewRetrierClient(c Client, options ...RetrierOption) Client {
	r := &retrierClient{c}
	for _, option := range options {
		option(r)
	}
	return r
}

type retrierClient struct {
	client Client
}

func (r *retrierClient) Do(req *http.Request) (*http.Response, error) {
	return nil, nil
}
