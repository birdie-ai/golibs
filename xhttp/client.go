package xhttp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/birdie-ai/golibs/slog"
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

	// RetrierOnRequestDoneFunc is the callback called when using [RetrierWithOnRequestDone].
	// The [*http.Request] is the original http request that just finished.
	// The [*http.Response] is the response of the request or nil if the request failed with an error (then the error will be non-nil).
	// The [error] is the request error, if the request failed, or nil if it succeeded (and the response will be non-nil).
	// The [time.Duration] is how long the http request took to be finished.
	// This is called for every request that is done, including retries.
	RetrierOnRequestDoneFunc func(req *http.Request, res *http.Response, err error, elapsed time.Duration)
)

const (
	// DefaultMinSleepPeriod is the min sleep period between retries (which is increased exponentially).
	DefaultMinSleepPeriod = 250 * time.Millisecond

	// DefaultMaxSleepPeriod is the max sleep period between retries.
	DefaultMaxSleepPeriod = 30 * time.Second
)

// NewRetrierClient wraps the given client with retry logic.
// The returned [Client] will automatically retry failed requests.
func NewRetrierClient(c Client, options ...RetrierOption) Client {
	r := &retrierClient{
		client:        c,
		sleep:         defaultSleep,
		minPeriod:     DefaultMinSleepPeriod,
		maxPeriod:     DefaultMaxSleepPeriod,
		onRequestDone: defaultOnRequestDone,
		retryStatusCodes: map[int]struct{}{
			http.StatusInternalServerError: {},
			http.StatusServiceUnavailable:  {},
		},
	}
	for _, option := range options {
		option(r)
	}
	return r
}

type (
	retrierClient struct {
		client           Client
		requestTimeout   time.Duration
		minPeriod        time.Duration
		maxPeriod        time.Duration
		checkResponse    bool
		sleep            func(context.Context, time.Duration)
		retryStatusCodes map[int]struct{}
		onRequestDone    RetrierOnRequestDoneFunc
	}
	readerCloserCanceller struct {
		io.ReadCloser
		cancel context.CancelFunc
	}
)

func (c *readerCloserCanceller) Close() error {
	c.cancel()
	return c.ReadCloser.Close()
}

func (r *retrierClient) Do(req *http.Request) (*http.Response, error) {
	var requestBody []byte

	if req.Body != nil {
		var err error
		requestBody, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
		if err := req.Body.Close(); err != nil {
			return nil, fmt.Errorf("closing request body: %w", err)
		}
	}

	return r.do(req.Context(), req, requestBody, r.minPeriod)
}

func (r *retrierClient) do(ctx context.Context, req *http.Request, requestBody []byte, sleepPeriod time.Duration) (*http.Response, error) {
	if ctx.Err() != nil {
		slog.FromCtx(ctx).Debug("xhttp.Client: stopping retry: parent context canceled", "error", ctx.Err())
		return nil, ctx.Err()
	}
	req, cancel := r.newRequest(ctx, req, requestBody)

	log := slog.FromCtx(ctx).With("request_url", req.URL)

	res, err := r.client.Do(req)
	// TODO: handle errors/responses and calculate proper elapsed
	r.onRequestDone(req, res, err, 0)
	if err != nil {
		cancel()

		// Sadly there is no other way to detect this error other than using the opaque string message
		// The error type is internal and the http pkg does not provide a way to check it
		// - https://cs.opensource.google/go/go/+/refs/tags/go1.21.4:src/net/http/h2_bundle.go;l=9250
		//
		// For connections reset...same problem:
		// - https://github.com/golang/go/blob/d0dc93c8e1a5be4e0a44b7f8ecb0cb1417de50ce/src/net/http/transport_test.go#L2207
		if errors.Is(err, context.DeadlineExceeded) ||
			strings.Contains(err.Error(), "http2: server sent GOAWAY and closed the connection") ||
			strings.HasSuffix(err.Error(), "i/o timeout") ||
			strings.HasSuffix(err.Error(), "connect: connection refused") ||
			strings.HasSuffix(err.Error(), "EOF") ||
			strings.HasSuffix(err.Error(), "write: broken pipe") ||
			strings.HasSuffix(err.Error(), "connection reset by peer") {

			log.Debug("xhttp.Client: retrying request with error", "error", err, "sleep_period", sleepPeriod.String())
			r.sleep(ctx, sleepPeriod)
			return r.do(ctx, req, requestBody, min(sleepPeriod*2, r.maxPeriod))
		}

		log.Debug("xhttp.Client: non recoverable error", "error", err)
		return nil, err
	}

	res.Body = &readerCloserCanceller{res.Body, cancel}

	_, isRetryCode := r.retryStatusCodes[res.StatusCode]
	if isRetryCode {
		log := slog.FromCtx(ctx).With("status_code", res.StatusCode, "sleep_period", sleepPeriod.String())
		if err := res.Body.Close(); err != nil {
			log.Debug("xhttp.Client: unable to close response body while retrying", "error", err)
		}
		log.Debug("xhttp.Client: retrying request with error status code")
		// Maybe add handling for Retry-After header, so far this seems to be enough
		r.sleep(ctx, sleepPeriod)
		return r.do(ctx, req, requestBody, min(sleepPeriod*2, r.maxPeriod))
	}

	if r.checkResponse {
		// assuming that res.Body is never nil (from http.Do docs):
		// "If the returned error is nil, the Response will contain a non-nil Body which the user is expected to close."
		log.Debug("xhttp.Client: checking response body")
		respBodyBytes, err := io.ReadAll(res.Body)
		if cerr := res.Body.Close(); cerr != nil {
			log.Debug("xhttp.Client: error closing response body", "error", cerr)
		}
		if err != nil {
			log.Debug("xhttp.Client: retrying request with error reading response body", "error", err)
			r.sleep(ctx, sleepPeriod)
			return r.do(ctx, req, requestBody, min(sleepPeriod*2, r.maxPeriod))
		}
		log.Debug("xhttp.Client: response body read with success")
		res.Body = io.NopCloser(bytes.NewReader(respBodyBytes))
	}

	return res, nil
}

func (r *retrierClient) newRequest(ctx context.Context, req *http.Request, requestBody []byte) (*http.Request, context.CancelFunc) {
	// We need to always guarantee that the request has a readable io.Reader for the original request body
	req.Body = io.NopCloser(bytes.NewReader(requestBody))
	if r.requestTimeout == 0 {
		return req, func() {}
	}
	newCtx, cancel := context.WithTimeout(ctx, r.requestTimeout)
	newReq := req.Clone(newCtx)
	return newReq, cancel
}

func defaultSleep(ctx context.Context, period time.Duration) {
	// Guarantee that we won't sleep more than the request context allows
	sleepCtx, cancel := context.WithTimeout(ctx, period)
	defer cancel()
	<-sleepCtx.Done()
}

func defaultOnRequestDone(*http.Request, *http.Response, error, time.Duration) {
}
