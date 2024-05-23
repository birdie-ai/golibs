package xhttp_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/birdie-ai/golibs/xhttp"
	"github.com/birdie-ai/golibs/xhttptest"
	"github.com/google/go-cmp/cmp"
)

func TestRetrierWithoutPerRequestTimeout(t *testing.T) {
	// With no per request timeout all requests must use the original request context
	fakeClient := xhttptest.NewClient()
	// here we test the proper request timeout being set by setting a very small timeout
	// per try/request and creating a request with no deadline at all, so we can check that the deadline exists
	client := xhttp.NewRetrierClient(fakeClient, noSleep())
	fakeClient.PushResponse(&http.Response{
		StatusCode: http.StatusServiceUnavailable,
	})
	fakeClient.PushError(retryableError())
	fakeClient.PushResponse(&http.Response{
		StatusCode: http.StatusOK,
	})

	// The request has no deadline by default. But individual requests must
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	request := newRequest(t, http.MethodPost, "/test", nil)
	request = request.Clone(ctx)
	res, err := client.Do(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got status %d; want %d", res.StatusCode, http.StatusOK)
	}

	requests := fakeClient.Requests()
	if len(requests) != 3 {
		t.Fatalf("got %d requests; want 3", len(requests))
	}

	for i, req := range requests {
		if req.Context() != ctx {
			t.Errorf("request %d got %v; want %v", i, req.Context(), ctx)
		}
	}
}

func TestRetrierPerRequestTryTimeout(t *testing.T) {
	t.Parallel()

	fakeClient := xhttptest.NewClient()
	const timeoutPerRequest = time.Millisecond
	// here we test the proper request timeout being set by setting a very small timeout
	// per try/request and creating a request with no deadline at all, so we can check that the deadline exists
	client := xhttp.NewRetrierClient(fakeClient, noSleep(), xhttp.RetrierWithRequestTimeout(timeoutPerRequest))
	fakeClient.PushError(retryableError())
	fakeClient.PushResponse(&http.Response{
		StatusCode: http.StatusServiceUnavailable,
	})
	fakeClient.PushResponse(&http.Response{
		StatusCode: http.StatusOK,
	})

	// The request has no deadline by default. But individual requests must
	request := newRequest(t, http.MethodGet, "/test", nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	request = request.Clone(ctx)
	res, err := client.Do(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got status %d; want %d", res.StatusCode, http.StatusOK)
	}

	requests := fakeClient.Requests()
	if len(requests) != 3 {
		t.Fatalf("got %d requests; want 3", len(requests))
	}

	firstReq := requests[0]
	firstDeadline, hasDeadline := firstReq.Context().Deadline()
	if !hasDeadline {
		t.Error("first request has no deadline")
	}

	secondReq := requests[1]
	secondDeadline, hasDeadline := secondReq.Context().Deadline()
	if !hasDeadline {
		t.Error("second request has no deadline")
	}
	if !secondDeadline.After(firstDeadline) {
		t.Errorf("want second deadline to be after first, got first deadline %v and second %v", firstDeadline, secondDeadline)
	}

	thirdReq := requests[2]
	thirdDeadline, hasDeadline := thirdReq.Context().Deadline()
	if !hasDeadline {
		t.Error("third request has no deadline")
	}
	if !thirdDeadline.After(secondDeadline) {
		t.Errorf("want third deadline to be after second, got second deadline %v and third %v", secondDeadline, thirdDeadline)
	}

	// This is not deterministic, but it is enough... I think :-)
	time.Sleep(10 * timeoutPerRequest)
	for i, req := range requests {
		if req.Context().Err() == nil {
			t.Fatalf("expected request %d to have expired context", i)
		}
	}

	// make sure that while we cancel created internal contexts we dont accidentally cancel the parent context
	if ctx.Err() != nil {
		t.Fatalf("want original context to be valid but got cancelled: %v", ctx.Err())
	}
}

func TestRetrierPerRequestTimeoutCancelPerReqContextOnError(t *testing.T) {
	t.Parallel()

	// Here we test that when we create contexts to be used on a per request basis they must not be cancelled
	// when we return the response. If we cancel the context inside the retry logic (the usual defer cancel())
	// the response body will fail when we try to read it (sometimes, cancellation is async...).
	const timeoutPerRequest = time.Hour
	fakeClient := xhttptest.NewClient()
	client := xhttp.NewRetrierClient(fakeClient, noSleep(), xhttp.RetrierWithRequestTimeout(timeoutPerRequest))
	fakeClient.PushError(errors.New("non recoverable error"))

	// The request has no deadline by default. But individual requests must
	request := newRequest(t, http.MethodGet, "/test", nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	request = request.Clone(ctx)
	res, err := client.Do(request)
	if err == nil {
		t.Fatalf("got response: %v want error", res)
	}
	requests := fakeClient.Requests()
	if len(requests) != 1 {
		t.Fatalf("got %d requests; want 1", len(requests))
	}

	// Guarantee that the context created for the request was cancelled
	req := requests[0]
	if req.Context().Err() == nil {
		t.Fatal("want request context to be cancelled after error response")
	}
	if ctx.Err() != nil {
		t.Fatal("parent context should not be cancelled after error response")
	}
}

func TestRetrierPerRequestTimeoutWontCancelContext(t *testing.T) {
	t.Parallel()

	// Here we test that when we create contexts to be used on a per request basis they must not be cancelled
	// when we return the response. If we cancel the context inside the retry logic (the usual defer cancel())
	// the response body will fail when we try to read it (sometimes, cancellation is async...).
	fakeClient := xhttptest.NewClient()
	const (
		timeoutPerRequest = time.Hour
		wantResponseBody  = "per request test body"
	)

	resBody := watchClose(strings.NewReader(wantResponseBody))
	client := xhttp.NewRetrierClient(fakeClient, noSleep(), xhttp.RetrierWithRequestTimeout(timeoutPerRequest))
	fakeClient.PushResponse(&http.Response{
		StatusCode: http.StatusOK,
		Body:       resBody,
	})

	// The request has no deadline by default. But individual requests must
	request := newRequest(t, http.MethodGet, "/test", nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	request = request.Clone(ctx)
	res, err := client.Do(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got status %d; want %d", res.StatusCode, http.StatusOK)
	}

	requests := fakeClient.Requests()
	if len(requests) != 1 {
		t.Fatalf("got %d requests; want 1", len(requests))
	}

	// Why we are using the request to test the context behavior ?
	// There is no way to get the context on the response, and yet the context used on the request
	// will control how long you have to read the response body, if the context used on the request
	// cancels then reading the response body will fail, so here we introspect into the requests
	// contexts to guarantee that the request context won't be cancelled before we read/close the response body.
	req := requests[0]
	if req.Context().Err() != nil {
		t.Fatalf("want request context to not be cancelled before closing response body, got: %v", req.Context().Err())
	}

	gotBody, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	assertEqual(t, string(gotBody), wantResponseBody)

	// after the response body is closed then the context created for the specific request should be cancelled
	if err := res.Body.Close(); err != nil {
		t.Fatalf("failed to close response body: %v", err)
	}
	// Ensure that on top of cancelling context we also actually call the underlying response body close method
	if resBody.CloseCalls != 1 {
		t.Fatalf("got %d Close calls; want 1", resBody.CloseCalls)
	}
	if req.Context().Err() == nil {
		t.Fatal("want request context to be cancelled after closing response body")
	}
	if ctx.Err() != nil {
		t.Fatal("parent context should not be cancelled after closing response body")
	}
}

func TestRetrierWithOnRequestDoneCallback(t *testing.T) {
	fakeClient := xhttptest.NewClient()
	gotRequests := []*http.Request{}
	gotResponses := []*http.Response{}
	gotElapsed := []time.Duration{}
	gotErrors := []error{}
	onRequestDone := func(req *http.Request, res *http.Response, err error, elapsed time.Duration) {
		gotRequests = append(gotRequests, req)
		gotResponses = append(gotResponses, res)
		gotElapsed = append(gotElapsed, elapsed)
		gotErrors = append(gotErrors, err)
	}
	sleep := func(context.Context, time.Duration) {}

	client := xhttp.NewRetrierClient(fakeClient,
		xhttp.RetrierWithOnRequestDone(onRequestDone),
		xhttp.RetrierWithSleep(sleep),
	)

	// Lets inject some controlled delay so we can test elapsed calculation
	const minDelay = 10 * time.Millisecond
	fakeClient.OnDo(func(*http.Request) {
		time.Sleep(minDelay)
	})
	firstResponse := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
	}
	fakeClient.PushResponse(firstResponse)
	wantErr := retryableError()
	fakeClient.PushError(wantErr)
	thirdResponse := &http.Response{
		StatusCode: http.StatusOK,
	}
	fakeClient.PushResponse(thirdResponse)

	request := newRequest(t, http.MethodGet, "/test", nil)
	res, err := client.Do(request)
	if err != nil {
		t.Fatalf("client.Do(%v) failed: %v", request, err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got status %v; want %v", res.StatusCode, http.StatusOK)
	}

	requestsMade := fakeClient.Requests()
	assertEqual(t, len(gotRequests), len(requestsMade))

	for i, got := range gotRequests {
		want := requestsMade[i]
		assertEqual(t, got.URL, want.URL)
		assertEqual(t, got.Method, want.Method)
		assertEqual(t, got.Header, want.Header)
	}

	wantErrs := []error{nil, wantErr, nil}
	assertEqual(t, len(gotErrors), len(wantErrs))
	for i, got := range gotErrors {
		want := wantErrs[i]
		if !errors.Is(got, want) {
			t.Errorf("got error %v; want %v", got, want)
		}
	}

	wantResponses := []*http.Response{firstResponse, nil, thirdResponse}
	assertEqual(t, len(gotResponses), len(wantResponses))
	for i, got := range gotResponses {
		want := wantResponses[i]
		if (want != nil) != (got != nil) {
			t.Errorf("got response %d %v; want %v", i, got, want)
			continue
		}
		if want == nil {
			continue
		}
		assertEqual(t, got.StatusCode, want.StatusCode)
	}

	for i, got := range gotElapsed {
		if got < minDelay {
			t.Errorf("elapsed on call[%d] %v is smaller than min delay %v on Do() call", i, got, minDelay)
		}
	}
}

func TestRetrierWithOnRetryCallback(t *testing.T) {
	fakeClient := xhttptest.NewClient()
	gotRetriedReqs := []*http.Request{}
	gotRetriedRes := []*http.Response{}
	gotRetriedErrs := []error{}
	onRetry := func(req *http.Request, res *http.Response, err error) {
		gotRetriedReqs = append(gotRetriedReqs, req)
		gotRetriedRes = append(gotRetriedRes, res)
		gotRetriedErrs = append(gotRetriedErrs, err)
	}
	sleep := func(context.Context, time.Duration) {}

	client := xhttp.NewRetrierClient(fakeClient,
		xhttp.RetrierWithOnRetry(onRetry),
		xhttp.RetrierWithSleep(sleep),
	)

	firstResponse := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
	}
	fakeClient.PushResponse(firstResponse)
	wantErr := retryableError()
	fakeClient.PushError(wantErr)
	thirdResponse := &http.Response{
		StatusCode: http.StatusOK,
	}
	fakeClient.PushResponse(thirdResponse)

	request := newRequest(t, http.MethodGet, "/test", nil)
	res, err := client.Do(request)
	if err != nil {
		t.Fatalf("client.Do(%v) failed: %v", request, err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got status %v; want %v", res.StatusCode, http.StatusOK)
	}

	// The last request is a success, it should not be included when comparing with retried requests
	requestsMade := fakeClient.Requests()[0:2]
	assertEqual(t, len(gotRetriedReqs), len(requestsMade))

	for i, got := range gotRetriedReqs {
		want := requestsMade[i]
		assertEqual(t, got.URL, want.URL)
		assertEqual(t, got.Method, want.Method)
		assertEqual(t, got.Header, want.Header)
	}

	wantErrs := []error{nil, wantErr}
	assertEqual(t, len(gotRetriedErrs), len(wantErrs))
	for i, got := range gotRetriedErrs {
		want := wantErrs[i]
		if !errors.Is(got, want) {
			t.Errorf("got error %v; want %v", got, want)
		}
	}

	wantResponses := []*http.Response{firstResponse, nil}
	assertEqual(t, len(gotRetriedRes), len(wantResponses))
	for i, got := range gotRetriedRes {
		want := wantResponses[i]
		if (want != nil) != (got != nil) {
			t.Errorf("got response %d %v; want %v", i, got, want)
			continue
		}
		if want == nil {
			continue
		}
		assertEqual(t, got.StatusCode, want.StatusCode)
	}
}

func TestRetrierExponentialBackoff(t *testing.T) {
	fakeClient := xhttptest.NewClient()
	gotSleepPeriods := []time.Duration{}
	gotContexts := []context.Context{}
	sleep := func(ctx context.Context, period time.Duration) {
		gotContexts = append(gotContexts, ctx)
		gotSleepPeriods = append(gotSleepPeriods, period)
	}

	client := xhttp.NewRetrierClient(fakeClient,
		xhttp.RetrierWithMinSleepPeriod(time.Second),
		xhttp.RetrierWithMaxSleepPeriod(10*time.Second),
		xhttp.RetrierWithSleep(sleep),
	)

	wantSleepPeriods := []time.Duration{
		time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		10 * time.Second,
		10 * time.Second,
	}

	for range 3 {
		// Interleave HTTP status errors with client errors that are retryable
		fakeClient.PushResponse(&http.Response{
			StatusCode: http.StatusServiceUnavailable,
		})
		fakeClient.PushError(retryableError())
	}
	fakeClient.PushResponse(&http.Response{
		StatusCode: http.StatusOK,
	})

	request := newRequest(t, http.MethodGet, "/test", nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	request = request.Clone(ctx)
	res, err := client.Do(request)
	if err != nil {
		t.Fatalf("client.Do(%v) failed: %v", request, err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got status %v; want %v", res.StatusCode, http.StatusOK)
	}

	requestsMade := len(fakeClient.Requests())
	if requestsMade != 7 {
		t.Fatalf("got %d requests; want 7", requestsMade)
	}

	assertEqual(t, gotSleepPeriods, wantSleepPeriods)
	for i, gotContext := range gotContexts {
		if gotContext != ctx {
			t.Errorf("got ctx[%d] %v != want %v", i, gotContext, ctx)
		}
	}
}

func TestRetrierWontRetryIfParentCtxExceeded(t *testing.T) {
	// Lets guarantee that we don't sleep at all when the parent context is cancelled using the default sleep implementation
	// This test will hang for an hour if the default behavior is broken.
	ctx, cancel := context.WithCancel(context.Background())
	fakeClient := xhttptest.NewClient()
	fakeClient.OnDo(func(*http.Request) {
		// When the Do call returns it will try to retry with the min sleep of an hour
		// but the parent context is cancelled, so it shouldn't sleep.
		cancel()
	})
	client := xhttp.NewRetrierClient(fakeClient, xhttp.RetrierWithMinSleepPeriod(time.Hour))

	fakeClient.PushError(context.DeadlineExceeded)

	const url = "http://test"
	request := newRequest(t, http.MethodGet, url, nil)
	request = request.Clone(ctx)
	_, err := client.Do(request)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got err %v; want %v", err, context.Canceled)
	}
	requests := fakeClient.Requests()
	if len(requests) != 1 {
		t.Fatalf("got %d requests; want 1", len(requests))
	}
}

func TestRetrierByDefaultWontRetryIfResponseBodyCantBeRead(t *testing.T) {
	fakeClient := xhttptest.NewClient()
	client := xhttp.NewRetrierClient(fakeClient, noSleep())

	fakeClient.PushResponse(&http.Response{
		Body:       &fakeReaderCloser{readErr: errors.New("fake error when reading")},
		StatusCode: http.StatusOK,
	})

	const url = "http://test"
	request := newRequest(t, http.MethodGet, url, nil)
	res, err := client.Do(request)
	if err != nil {
		t.Fatalf("got err %v; want nil", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got status %d; want %d", res.StatusCode, http.StatusOK)
	}
	requests := fakeClient.Requests()
	if len(requests) != 1 {
		t.Fatalf("got %d requests; want 1", len(requests))
	}
}

func TestRetrierRetryOnFailedResponseRead(t *testing.T) {
	const wantBody = "successfully read response body !!"
	fakeClient := xhttptest.NewClient()
	client := xhttp.NewRetrierClient(fakeClient, noSleep(), xhttp.RetrierWithRespCheck())

	fakeClient.PushResponse(&http.Response{
		Body:       &fakeReaderCloser{readErr: errors.New("fake error")},
		StatusCode: http.StatusOK,
	})
	fakeClient.PushResponse(&http.Response{
		Body:       io.NopCloser(strings.NewReader(wantBody)),
		StatusCode: http.StatusOK,
	})

	const url = "http://test"
	request := newRequest(t, http.MethodGet, url, nil)
	res, err := client.Do(request)
	if err != nil {
		t.Fatalf("got err %v; want nil", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got status %d; want %d", res.StatusCode, http.StatusOK)
	}
	gotResp, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("got err %v reading response body", err)
	}

	assertEqual(t, string(gotResp), wantBody)

	requests := fakeClient.Requests()
	if len(requests) != 2 {
		t.Fatalf("got %d requests; want 2", len(requests))
	}
}

func TestRetrierRetrySpecificErrors(t *testing.T) {
	// This handles errors caught in production related to connection failing and other specific errors
	// like HTTP2 errors. Sadly we didn't find a more programatic way to detect these errors besides
	// inspecting the error string.
	retryErrors := []error{
		errors.New("<specific details> http2: server sent GOAWAY and closed the connection <specific details>"),
		errors.New("<specific details>: connection reset by peer"),
		errors.New("<specific details>: i/o timeout"),
		errors.New("<specific details>: connect: connection refused"),
		errors.New("<specific details>: EOF"),
		errors.New("<specific details>: write: broken pipe"),
		context.DeadlineExceeded,
	}
	for _, retryError := range retryErrors {
		t.Run(retryError.Error(), func(t *testing.T) {
			fakeClient := xhttptest.NewClient()
			client := xhttp.NewRetrierClient(fakeClient, noSleep())

			fakeClient.PushError(retryError)
			fakeClient.PushResponse(&http.Response{
				StatusCode: http.StatusOK,
			})

			const url = "http://test"
			request := newRequest(t, http.MethodGet, url, nil)
			res, err := client.Do(request)
			if err != nil {
				t.Fatalf("got err %v; want nil", err)
			}
			if res.StatusCode != http.StatusOK {
				t.Fatalf("got status %d; want %d", res.StatusCode, http.StatusOK)
			}
			requests := fakeClient.Requests()
			if len(requests) != 2 {
				t.Fatalf("got %d requests; want 2", len(requests))
			}
		})
	}
}

func TestWontRetryClientErrors(t *testing.T) {
	fakeClient := xhttptest.NewClient()
	client := xhttp.NewRetrierClient(fakeClient, noSleep())

	wantErr := errors.New("some error")
	fakeClient.PushError(wantErr)

	const url = "http://test"
	request := newRequest(t, http.MethodGet, url, nil)
	_, err := client.Do(request)
	if !errors.Is(err, wantErr) {
		t.Fatal(err)
	}
	requests := fakeClient.Requests()
	if len(requests) != 1 {
		t.Fatalf("got %d requests; want 1", len(requests))
	}
}

func TestRetrierRetryStatusCodes(t *testing.T) {
	// Default status codes that are always retried
	retryStatusCodes := []int{
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
	}
	testRetry := func(t *testing.T, fakeClient *xhttptest.Client, client xhttp.Client, wantMethod string, wantStatus int, withBody bool) {
		const wantPath = "/test/retry"

		failedRespBody := &fakeReaderCloser{}
		successRespBody := &fakeReaderCloser{}

		fakeClient.PushResponse(&http.Response{
			StatusCode: wantStatus,
			Body:       failedRespBody,
		})
		fakeClient.PushResponse(&http.Response{
			StatusCode: http.StatusOK,
			Body:       successRespBody,
		})

		url := "http://test" + wantPath

		var (
			wantBody string
			request  *http.Request
		)

		if withBody {
			wantBody = t.Name()
			request = newRequest(t, wantMethod, url, []byte(wantBody))
		} else {
			request = newRequest(t, wantMethod, url, nil)
		}

		res, err := client.Do(request)
		if err != nil {
			t.Fatalf("client.Do(%v) failed: %v", request, err)
		}
		if res.StatusCode != http.StatusOK {
			t.Fatalf("got status %v; want %v", res.StatusCode, http.StatusOK)
		}

		// Here we guarantee that retried requests will have their responses body closed
		if failedRespBody.closeCalls != 1 {
			t.Fatalf("got %d Close calls on the failed response, want 1", failedRespBody.closeCalls)
		}
		if successRespBody.closeCalls != 0 {
			t.Fatalf("got %d Close calls on the success response, want 0", successRespBody.closeCalls)
		}

		assertReq := func(req *http.Request) {
			t.Helper()

			if req.URL.Path != wantPath {
				t.Errorf("got path %q; want %q", req.URL.Path, wantPath)
			}
			if req.Method != wantMethod {
				t.Errorf("got method %q; want %q", req.Method, wantMethod)
			}
			// Each request made must have an independent/fully readable body
			if withBody {
				reqBody, err := io.ReadAll(req.Body)
				if err != nil {
					t.Errorf("reading request body %v", err)
				}
				gotBody := string(reqBody)
				assertEqual(t, gotBody, wantBody)
			}
		}

		requests := fakeClient.Requests()
		if len(requests) != 2 {
			t.Fatalf("got %d requests; want 2", len(requests))
		}

		assertReq(requests[0])
		assertReq(requests[1])
	}
	for _, wantStatus := range retryStatusCodes {
		for _, wantMethod := range httpMethods() {

			t.Run(fmt.Sprintf("%s %d", wantMethod, wantStatus), func(t *testing.T) {
				t.Run("with body", func(t *testing.T) {
					fakeClient := xhttptest.NewClient()
					client := xhttp.NewRetrierClient(fakeClient, noSleep())
					testRetry(t, fakeClient, client, wantMethod, wantStatus, true)
				})
				t.Run("no body", func(t *testing.T) {
					fakeClient := xhttptest.NewClient()
					client := xhttp.NewRetrierClient(fakeClient, noSleep())
					testRetry(t, fakeClient, client, wantMethod, wantStatus, false)
				})
			})
		}
	}

	for _, wantMethod := range httpMethods() {
		t.Run("configuring customized retry status code", func(t *testing.T) {
			wantStatus := []int{
				http.StatusConflict,
				http.StatusTooManyRequests,
			}
			for _, w := range wantStatus {
				t.Run("with body", func(t *testing.T) {
					fakeClient := xhttptest.NewClient()
					client := xhttp.NewRetrierClient(fakeClient, noSleep(), xhttp.RetrierWithStatuses(wantStatus...))
					testRetry(t, fakeClient, client, wantMethod, w, true)
				})
				t.Run("no body", func(t *testing.T) {
					fakeClient := xhttptest.NewClient()
					client := xhttp.NewRetrierClient(fakeClient, noSleep(), xhttp.RetrierWithStatuses(wantStatus...))
					testRetry(t, fakeClient, client, wantMethod, w, false)
				})
			}
		})
	}
}

func TestRetrierNoRetryStatusCodes(t *testing.T) {
	for wantStatus := 200; wantStatus < 500; wantStatus++ {
		if wantStatus == http.StatusTooManyRequests {
			continue
		}
		for _, wantMethod := range httpMethods() {

			t.Run(fmt.Sprintf("%s %d", wantMethod, wantStatus), func(t *testing.T) {
				fakeClient := xhttptest.NewClient()
				client := xhttp.NewRetrierClient(fakeClient, noSleep())
				wantPath := "/" + t.Name()

				fakeClient.PushResponse(&http.Response{StatusCode: wantStatus})

				url := "http://testing" + wantPath
				request := newRequest(t, wantMethod, url, nil)

				res, err := client.Do(request)
				if err != nil {
					t.Fatalf("client.Do(%v) failed: %v", request, err)
				}
				if res.StatusCode != wantStatus {
					t.Fatalf("got status %v; want %v", res.StatusCode, wantStatus)
				}
				requests := fakeClient.Requests()
				if len(requests) != 1 {
					t.Fatalf("got %d requests; want 1", len(requests))
				}
				req := requests[0]
				if req.URL.Path != wantPath {
					t.Errorf("got path %q; want %q", req.URL.Path, wantPath)
				}
				if req.Method != wantMethod {
					t.Errorf("got method %q; want %q", req.Method, wantMethod)
				}
			})
		}
	}
}

func TestReadingRequestBodyFails(t *testing.T) {
	fakeClient := xhttptest.NewClient()
	client := xhttp.NewRetrierClient(fakeClient, noSleep())
	wantErr := errors.New("fake read error")

	const url = "http://testing"
	fakeReader := &fakeReaderCloser{
		readErr: wantErr,
	}
	request, err := http.NewRequest(http.MethodPost, url, fakeReader)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Do(request)
	if !errors.Is(err, wantErr) {
		t.Errorf("got err %v; want %v", err, wantErr)
	}
	got := fakeClient.Requests()
	if len(got) > 0 {
		t.Errorf("unexpected requests: %v", got)
	}
}

func TestClosingRequestBodyFails(t *testing.T) {
	fakeClient := xhttptest.NewClient()
	client := xhttp.NewRetrierClient(fakeClient, noSleep())
	wantErr := errors.New("fake close error")

	const url = "http://testing"
	fakeReader := &fakeReaderCloser{
		readErr:  io.EOF,
		closeErr: wantErr,
	}
	request, err := http.NewRequest(http.MethodPost, url, fakeReader)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Do(request)
	if !errors.Is(err, wantErr) {
		t.Errorf("got err %v; want %v", err, wantErr)
	}
	got := fakeClient.Requests()
	if len(got) > 0 {
		t.Errorf("unexpected requests: %v", got)
	}
}

func TestNoRequestSentIfContextIsCancelled(t *testing.T) {
	fakeClient := xhttptest.NewClient()
	client := xhttp.NewRetrierClient(fakeClient, noSleep())

	const url = "http://testing"
	request := newRequest(t, http.MethodPost, url, nil)
	ctx, cancel := context.WithCancel(context.Background())
	request = request.Clone(ctx)
	cancel()

	_, err := client.Do(request)
	wantErr := context.Canceled
	if !errors.Is(err, wantErr) {
		t.Errorf("got err %v; want %v", err, wantErr)
	}
	got := fakeClient.Requests()
	if len(got) > 0 {
		t.Errorf("unexpected requests: %v", got)
	}
}

type fakeReaderCloser struct {
	readErr    error
	closeErr   error
	closeCalls int
}

func (f *fakeReaderCloser) Read([]byte) (int, error) {
	return 0, f.readErr
}

func (f *fakeReaderCloser) Close() error {
	f.closeCalls++
	return f.closeErr
}

func newRequest(t *testing.T, method, url string, body []byte) *http.Request {
	t.Helper()

	// In order to test some scenarios we need to build requests with no body
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}

	request, err := http.NewRequest(method, url, r)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func httpMethods() []string {
	return []string{
		http.MethodPost,
		http.MethodGet,
		http.MethodPut,
		http.MethodHead,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodTrace,
		http.MethodConnect,
		http.MethodOptions,
	}

}

type closeWatcher struct {
	io.Reader
	CloseCalls int
}

func (c *closeWatcher) Close() error {
	c.CloseCalls++
	return nil
}

func watchClose(r io.Reader) *closeWatcher {
	return &closeWatcher{r, 0}
}

func noSleep() xhttp.RetrierOption {
	return xhttp.RetrierWithSleep(func(context.Context, time.Duration) {})
}

func assertEqual[T any](t *testing.T, got T, want T) {
	t.Helper()

	if diff := cmp.Diff(got, want); diff != "" {
		t.Logf("got: %v", got)
		t.Logf("want: %v", want)
		t.Fatalf("diff: %v", diff)
	}
}

func retryableError() error {
	return errors.New("http2: server sent GOAWAY and closed the connection")
}

func TestParseRetryAfter(t *testing.T) {
	cases := []struct {
		value string
		d     time.Duration
		tm    time.Time
	}{
		{value: ""},
		{value: "123", d: 123 * time.Second},
		{value: "Wed, 21 Oct 2015 07:28:00 GMT", tm: time.Date(2015, 10, 21, 7, 28, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		d, tm, err := xhttp.ParseRetryAfter(c.value)
		if err != nil {
			t.Errorf("ParseRetryAfter(%q) returned error: %v", c.value, err)
		} else if !(d == c.d && tm.Equal(c.tm)) {
			t.Errorf("ParseRetryAfter(%q) == (%v, %v, nil), want (%v, %v, nil)",
				c.value,
				d, tm,
				c.d, c.tm)
		}
	}
}
