package xhttp

import (
	"context"
	"time"
)

// RetrierWithOnRetry configures a callback function that will be called for each request retry.
// It includes only retried requests. The callback is called after a response/error is received
// and it is decided that the request is a retry-able failure but before the request is actually retried.
// The callback is called from the same goroutine that called the retrier Do method.
func RetrierWithOnRetry(f RetrierOnRetryFunc) RetrierOption {
	return func(r *retrierClient) {
		r.onRetry = f
	}
}

// RetrierWithOnRequestDone configures a callback function that will be called for each request done by the retrier.
// This includes retried requests. The callback is called after a response/error is received but before the response/error is processed (for retrying).
// The callback is called from the same goroutine that called the retrier Do method.
func RetrierWithOnRequestDone(f RetrierOnRequestDoneFunc) RetrierOption {
	return func(r *retrierClient) {
		r.onRequestDone = f
	}
}

// RetrierWithJitter configures the maximum jitter between retries. When calculating the retry period
// based on min/max/exponential increase a random jitter will be added on top of
// the retry period using this jitter duration as the random max.
// This is useful to avoid bursts of retries, which even as they increase exponentially will
// be made on a very similar cadence if the initial request happens at the same time.
func RetrierWithJitter(jitter time.Duration) RetrierOption {
	return func(r *retrierClient) {
		r.jitter = jitter
	}
}

// RetrierWithMinSleepPeriod configures the min period that the retrier will sleep between retries.
// The retrier uses an exponential backoff, so this will be only the initial sleep period, that then grows exponentially.
// If not defined it will default [DefaultMinSleepPeriod].
func RetrierWithMinSleepPeriod(minPeriod time.Duration) RetrierOption {
	return func(r *retrierClient) {
		r.minPeriod = minPeriod
	}
}

// RetrierWithMaxSleepPeriod configures the max period that the retrier will sleep between retries.
// If not defined it will default [DefaultMaxSleepPeriod].
func RetrierWithMaxSleepPeriod(maxPeriod time.Duration) RetrierOption {
	return func(r *retrierClient) {
		r.maxPeriod = maxPeriod
	}
}

// RetrierWithRespCheck configures the retrier to read the responses of successful HTTP requests and retry
// if reading the response fails (like the connection dropping during the response transmission).
// Beware that this option involves reading the entire response body in memory, it is not a good idea to use this with streams.
func RetrierWithRespCheck() RetrierOption {
	return func(r *retrierClient) {
		r.checkResponse = true
	}
}

// RetrierWithSleep configures the sleep function used to sleep between retries, usually used for testing.
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
// This is useful for situations where the service where the request is sent is hanging forever but only on some requests.
// On such a situation you can have two timeouts. One provided on the request passed to [Client.Do] on the request context and the timeout
// defined with this option. Lets say the overall timeout is 10 min (when you created the original request) and this configuration here is
// 30 secs. Now every 30 sec the request will fail since it timeouted and will be retried, until the parent timeout of 10 min expires.
func RetrierWithRequestTimeout(timeout time.Duration) RetrierOption {
	return func(r *retrierClient) {
		r.requestTimeout = timeout
	}
}

// RetrierWithStatuses will configure the retrier to retry when these specific status code are received.
// This option only adds more status codes that will be retried, it will still retry on default error status codes
// like [http.StatusServiceUnavailable] and [http.StatusInternalServerError]
func RetrierWithStatuses(statuses ...int) RetrierOption {
	return func(r *retrierClient) {
		for _, status := range statuses {
			r.retryStatusCodes[status] = struct{}{}
		}
	}
}
