package xhttp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type (
	// Client abstracts a [http.Client], allowing us to create wrappers for http clients adding useful
	// functionality like retry and metrics. It has the same API as [http.Client] and is intended to be
	// a drop-in replacement (but not all methods are supported yet).
	// The client will have to read the entire request body in memory in order to properly retry a failed request.
	Client interface {
		Do(req *http.Request) (*http.Response, error)
	}
	// RetrierOption is used to configure retrier clients created with [NewRetrierClient].
	RetrierOption func(*retrierClient)
)

// RetrierWithMinSleepPeriod configures the min period that the retrier will sleep between retries.
// Retrying uses an exponential backoff, so this will be only the initial sleep period, that then grows exponentially.
// If not defined it will default to a second.
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
// A context is created with this timeout and the original request context as parent for each request.
//
// This is useful for situation where the service where the request is sent is hanging forever but only on some requests (for some reason).
// On such a situation you can have two timeouts. One provided on the request passed to [Client.Do] on the request context and the timeout
// defined with this option. Lets say the overall timeout is 10 min (when you created the original request) and this configuration here is
// 30 secs. Now every 30 sec the request will fail since it timeouted and will be retried, until the parent timeout of 10 min expires.
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
		retryStatusCodes: map[int]struct{}{
			http.StatusInternalServerError: {},
			http.StatusServiceUnavailable:  {},
			http.StatusTooManyRequests:     {},
		},
	}
	for _, option := range options {
		option(r)
	}
	return r
}

type retrierClient struct {
	client           Client
	requestTimeout   time.Duration
	minPeriod        time.Duration
	sleep            func(context.Context, time.Duration)
	retryStatusCodes map[int]struct{}
}

func (r *retrierClient) Do(req *http.Request) (*http.Response, error) {
	requestBody, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("reading request body: %w", err)
	}
	if err := req.Body.Close(); err != nil {
		return nil, fmt.Errorf("closing request body: %w", err)
	}
	return r.do(req.Context(), req, requestBody, r.minPeriod)
}

func (r *retrierClient) do(ctx context.Context, req *http.Request, requestBody []byte, sleepPeriod time.Duration) (*http.Response, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	req, cancel := r.newRequest(ctx, req, requestBody)
	defer cancel()

	res, err := r.client.Do(req)
	if err != nil {
		// Sadly there is no other way to detect this error other than using the opaque string message
		// The error type is internal and the http pkg does not provide a way to check it
		// - https://cs.opensource.google/go/go/+/refs/tags/go1.21.4:src/net/http/h2_bundle.go;l=9250
		//
		// For connections reset...same problem:
		// - https://github.com/golang/go/blob/d0dc93c8e1a5be4e0a44b7f8ecb0cb1417de50ce/src/net/http/transport_test.go#L2207
		// We need to go for some suffix matches.
		if strings.Contains(err.Error(), "http2: server sent GOAWAY and closed the connection") ||
			strings.HasSuffix(err.Error(), ": connection reset by peer") {

			r.sleep(ctx, sleepPeriod)
			return r.do(ctx, req, requestBody, sleepPeriod*2)
		}
		return nil, err
	}

	_, isRetryCode := r.retryStatusCodes[res.StatusCode]
	if isRetryCode {
		// Maybe add handling for Retry-After header, so far this seems to be enough
		r.sleep(ctx, sleepPeriod)
		return r.do(ctx, req, requestBody, sleepPeriod*2)
	}

	return res, nil
}

func (r *retrierClient) newRequest(ctx context.Context, req *http.Request, requestBody []byte) (*http.Request, context.CancelFunc) {
	reqCtx := ctx
	cancel := func() {}

	if r.requestTimeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, r.requestTimeout)
	}
	newReq := req.Clone(reqCtx)
	// We need to always guarantee that the request has a readable io.Reader for the original request body
	newReq.Body = io.NopCloser(bytes.NewReader(requestBody))
	return newReq, cancel
}

func defaultSleep(ctx context.Context, period time.Duration) {
	// Guarantee that we won't sleep more than the request context allows
	sleepCtx, cancel := context.WithTimeout(ctx, period)
	defer cancel()
	<-sleepCtx.Done()
}
