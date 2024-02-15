package xhttp

import (
	"net/http"
	"time"
)

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

// RetrierWithMinPeriod configures the min period that the retrier will sleep between retries.
// Retrying uses an exponential backoff, so this will be only the initial sleep period, that then grows exponentially.
func RetrierWithMinPeriod(minPeriod time.Duration) RetrierOption {
	return func(r *retrierClient) {
		r.minPeriod = minPeriod
	}
}

// RetrierWithSleep configures the sleep function used to sleep between retries, usually used for testing.
// But can be used as a way to measure how much retries happened since this is called before each retry.
func RetrierWithSleep(sleep func(time.Duration)) RetrierOption {
	return func(r *retrierClient) {
		r.sleep = sleep
	}
}

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
	r := &retrierClient{
		client:    c,
		sleep:     time.Sleep,
		minPeriod: time.Second,
	}
	for _, option := range options {
		option(r)
	}
	return r
}

type retrierClient struct {
	client    Client
	minPeriod time.Duration
	sleep     func(time.Duration)
}

func (r *retrierClient) Do(req *http.Request) (*http.Response, error) {
	return r.do(req, r.minPeriod)
}

func (r *retrierClient) do(req *http.Request, sleepPeriod time.Duration) (*http.Response, error) {
	// TODO: do actual retries :-)
	res, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}

	// TODO: improve retry logic
	if res.StatusCode == http.StatusInternalServerError ||
		res.StatusCode == http.StatusServiceUnavailable ||
		res.StatusCode == http.StatusTooManyRequests {
		r.sleep(sleepPeriod)
		return r.do(req, sleepPeriod*2)
	}

	return res, nil
}
