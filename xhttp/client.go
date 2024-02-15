package xhttp

import (
	"context"
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

// RetrierWithMinSleepPeriod configures the min period that the retrier will sleep between retries.
// Retrying uses an exponential backoff, so this will be only the initial sleep period, that then grows exponentially.
func RetrierWithMinSleepPeriod(minPeriod time.Duration) RetrierOption {
	return func(r *retrierClient) {
		r.minPeriod = minPeriod
	}
}

// RetrierWithSleep configures the sleep function used to sleep between retries, usually used for testing.
// But can be used as a way to measure how much retries happened since this is called before each retry.
func RetrierWithSleep(sleep func(context.Context, time.Duration)) RetrierOption {
	return func(r *retrierClient) {
		r.sleep = sleep
	}
}

// RetrierWithRequestTimeout configures a client retrier with the given timeout. This timeout is used per request/try.
// When calling [Client.Do] if the request has a context with a deadline longer than this timeout the retrier
// will keep retrying until the parent request context is cancelled/deadline expires.
// If this timeout is bigger than the deadline of the request context then the request context will be respected
// (a context is created with this timeout and the request context as parent).
func RetrierWithRequestTimeout(timeout time.Duration) RetrierOption {
	return func(r *retrierClient) {
		r.requestTimeout = timeout
	}
}

// NewRetrierClient wraps the given client with retry logic.
// The returned [Client] will automatically retry failed requests.
func NewRetrierClient(c Client, options ...RetrierOption) Client {
	r := &retrierClient{
		client:    c,
		sleep:     defaultSleep,
		minPeriod: time.Second,
	}
	for _, option := range options {
		option(r)
	}
	return r
}

type retrierClient struct {
	client         Client
	requestTimeout time.Duration
	minPeriod      time.Duration
	sleep          func(context.Context, time.Duration)
}

func (r *retrierClient) Do(req *http.Request) (*http.Response, error) {
	// We need to keep the original request context while we retry since we create
	// new requests recursively as we retry.
	return r.do(req.Context(), req, r.minPeriod)
}

func (r *retrierClient) do(ctx context.Context, req *http.Request, sleepPeriod time.Duration) (*http.Response, error) {
	req, cancel := r.newRequest(ctx, req)
	defer cancel()

	res, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}

	// TODO: improve retry logic
	if res.StatusCode == http.StatusInternalServerError ||
		res.StatusCode == http.StatusServiceUnavailable ||
		res.StatusCode == http.StatusTooManyRequests {

		r.sleep(ctx, sleepPeriod)

		req, cancel := r.newRequest(ctx, req)
		defer cancel()

		return r.do(ctx, req, sleepPeriod*2)
	}

	return res, nil
}

func (r *retrierClient) newRequest(ctx context.Context, req *http.Request) (*http.Request, context.CancelFunc) {
	if r.requestTimeout == 0 {
		return req, func() {}
	}
	reqCtx, cancel := context.WithTimeout(ctx, r.requestTimeout)
	req = req.Clone(reqCtx)
	return req, cancel
}

func defaultSleep(ctx context.Context, period time.Duration) {
	// Guarantee that we won't sleep more than the request context allows
	sleepCtx, cancel := context.WithTimeout(ctx, period)
	defer cancel()
	<-sleepCtx.Done()
}
