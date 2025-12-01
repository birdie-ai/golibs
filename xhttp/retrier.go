package xhttp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
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
	// The [*http.Response] is the response returned by the [Client.Do] call.
	// The [error] is the response error returned by the [Client.Do] call.
	// The [time.Duration] is how long the http request took to be finished.
	// This is called for every request that is done, including retries.
	RetrierOnRequestDoneFunc func(req *http.Request, res *http.Response, err error, elapsed time.Duration)

	// RetrierOnRetryFunc is the callback called when using [RetrierWithOnRetry].
	// The [*http.Request] is the original http request that just finished.
	// The [*http.Response] is the response returned by the [Client.Do] call.
	// The [error] is the response error returned by the [Client.Do] call.
	// This is called every time a request is retried (it failed and retrier decided to retry it).
	// The callback can read the [http.Response] body if it wants, for debugging for example, but
	// it doesn't have to close the body, the retries always close the response bodies when retrying.
	RetrierOnRetryFunc func(req *http.Request, res *http.Response, err error)
)

const (
	// DefaultMinSleepPeriod is the default min sleep period between retries (which is increased exponentially).
	DefaultMinSleepPeriod = 250 * time.Millisecond

	// DefaultMaxSleepPeriod is the default max sleep period between retries.
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
		onRetry:       defaultOnRetry,
		retryStatusCodes: map[int]struct{}{
			http.StatusTooManyRequests:     {},
			http.StatusInternalServerError: {},
			http.StatusBadGateway:          {},
			http.StatusServiceUnavailable:  {},
			http.StatusGatewayTimeout:      {},
		},
	}
	for _, option := range options {
		option(r)
	}
	return r
}

// ParseRetryAfter parses the Retry-After header in the response.
func ParseRetryAfter(value string) (time.Duration, time.Time, error) {
	if value == "" {
		return 0, time.Time{}, nil
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second, time.Time{}, nil
	}
	if t, err := http.ParseTime(value); err == nil {
		return 0, t, nil
	}
	return 0, time.Time{}, fmt.Errorf("invalid Retry-After header in http response: %s", value)
}

type (
	retrierClient struct {
		client           Client
		requestTimeout   time.Duration
		minPeriod        time.Duration
		maxPeriod        time.Duration
		jitter           time.Duration
		checkResponse    bool
		sleep            func(context.Context, time.Duration)
		retryStatusCodes map[int]struct{}
		onRequestDone    RetrierOnRequestDoneFunc
		onRetry          RetrierOnRetryFunc
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

	return r.do(req.Context(), req, requestBody)
}

func (r *retrierClient) do(ctx context.Context, req *http.Request, requestBody []byte) (*http.Response, error) {
	sleepPeriod := r.minPeriod
	incrementSleepPeriod := func() {
		sleepPeriod = min(sleepPeriod*2, r.maxPeriod)
	}

	for ctx.Err() == nil {
		req, cancel := r.newRequest(ctx, req, requestBody)
		log := slog.FromCtx(ctx).With("request_url", req.URL)
		start := time.Now()

		res, err := r.client.Do(req)
		r.onRequestDone(req, res, err, time.Since(start))
		if err != nil {
			cancel()

			// Sadly there is no other way to detect this error other than using the opaque string message
			// The error type is internal and the http pkg does not provide a way to check it
			// - https://cs.opensource.google/go/go/+/refs/tags/go1.21.4:src/net/http/h2_bundle.go;l=9250
			//
			// For connections reset... Same problem:
			// - https://github.com/golang/go/blob/d0dc93c8e1a5be4e0a44b7f8ecb0cb1417de50ce/src/net/http/transport_test.go#L2207
			emsg := err.Error()
			if errors.Is(err, context.DeadlineExceeded) ||
				strings.Contains(emsg, "http2: server sent GOAWAY and closed the connection") ||
				strings.HasSuffix(emsg, "i/o timeout") ||
				strings.HasSuffix(emsg, "read: connection timed out") ||
				strings.HasSuffix(emsg, "connect: connection refused") ||
				strings.HasSuffix(emsg, "EOF") ||
				strings.HasSuffix(emsg, "write: broken pipe") ||
				strings.HasSuffix(emsg, "connection reset by peer") ||
				strings.HasSuffix(emsg, "server closed idle connection") ||
				strings.HasSuffix(emsg, "use of closed network connection") ||
				strings.HasSuffix(emsg, "Temporary failure in name resolution") ||
				strings.HasSuffix(emsg, "cannot assign requested address") {

				log.Debug("xhttp.Client: retrying request with error", "error", err,
					"jitter", r.jitter.String(), "sleep_period", sleepPeriod.String())

				r.onRetry(req, res, err)
				r.sleep(ctx, r.addJitter(sleepPeriod))
				incrementSleepPeriod()
				continue
			}

			log.Debug("xhttp.Client: non recoverable error", "error", err)
			return nil, err
		}

		res.Body = &readerCloserCanceller{res.Body, cancel}

		_, isRetryCode := r.retryStatusCodes[res.StatusCode]
		if isRetryCode {
			r.onRetry(req, res, err)

			log := slog.FromCtx(ctx).With("status_code", res.StatusCode,
				"jitter", r.jitter.String(),
				"sleep_period", sleepPeriod.String())

			// Caller might have read the response body on onRetry, just close the body the now.
			if err := res.Body.Close(); err != nil {
				log.Debug("xhttp.Client: unable to close response body while retrying", "error", err)
			}
			log.Debug("xhttp.Client: retrying request with error status code")

			// handle Retry-After header
			const minRetryAfterDuration = time.Second
			retryAfter := res.Header.Get("Retry-After")
			requestedDuration, requestedTime, err := ParseRetryAfter(retryAfter)
			switch {
			case err != nil:
				log.Warn("xhttp.Client: parsing Retry-After header", "error", err)

			case requestedDuration >= minRetryAfterDuration:
				log.Debug("xhttp.Client: following Retry-After header", "duration", requestedDuration)
				sleepPeriod = min(requestedDuration, r.maxPeriod)

			case !requestedTime.IsZero():
				calculatedDuration := time.Until(requestedTime)
				if calculatedDuration >= minRetryAfterDuration {
					log.Debug("xhttp.Client: following Retry-After header", "time", requestedTime,
						"calculated_duration", calculatedDuration)
					sleepPeriod = min(calculatedDuration, r.maxPeriod)
				}
			}

			r.sleep(ctx, r.addJitter(sleepPeriod))
			incrementSleepPeriod()
			continue
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
				r.sleep(ctx, r.addJitter(sleepPeriod))
				incrementSleepPeriod()
				continue
			}
			log.Debug("xhttp.Client: response body read with success")
			res.Body = io.NopCloser(bytes.NewReader(respBodyBytes))
		}

		return res, nil
	}

	slog.FromCtx(ctx).Debug("xhttp.Client: stopping retry: parent context canceled", "error", ctx.Err())
	return nil, ctx.Err()
}

func (r *retrierClient) addJitter(v time.Duration) time.Duration {
	if r.jitter == 0 {
		return v
	}
	return v + time.Duration(rand.Int64N(int64(r.jitter)))
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

func defaultOnRetry(*http.Request, *http.Response, error) {
}
