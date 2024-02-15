package xhttp_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/birdie-ai/golibs/xhttp"
	"github.com/google/go-cmp/cmp"
)

func TestRetrierWithoutPerRequestTimeout(t *testing.T) {
	// With no per request timeout all requests must use the original request context
	fakeClient := NewFakeClient()
	const timeoutPerRequest = time.Millisecond
	// here we test the proper request timeout being set by setting a very small timeout
	// per try/request and creating a request with no deadline at all, so we can check that the deadline exists
	client := xhttp.NewRetrierClient(fakeClient, noSleep())
	fakeClient.PushResponse(&http.Response{
		StatusCode: http.StatusServiceUnavailable,
	})
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
	if len(requests) != 2 {
		t.Fatalf("got %d requests; want %d", len(requests), 2)
	}

	firstReq := requests[0]
	if firstReq.Context() != ctx {
		t.Errorf("got %v; want %v", firstReq.Context(), ctx)
	}

	secondReq := requests[1]
	if secondReq.Context() != ctx {
		t.Errorf("got %v; want %v", secondReq.Context(), ctx)
	}
}

func TestRetrierPerRequestTryTimeout(t *testing.T) {
	fakeClient := NewFakeClient()
	const timeoutPerRequest = time.Millisecond
	// here we test the proper request timeout being set by setting a very small timeout
	// per try/request and creating a request with no deadline at all, so we can check that the deadline exists
	client := xhttp.NewRetrierClient(fakeClient, noSleep(), xhttp.RetrierWithRequestTimeout(timeoutPerRequest))
	fakeClient.PushResponse(&http.Response{
		StatusCode: http.StatusServiceUnavailable,
	})
	fakeClient.PushResponse(&http.Response{
		StatusCode: http.StatusOK,
	})

	// The request has no deadline by default. But individual requests must
	request := newRequest(t, http.MethodGet, "/test", nil)
	res, err := client.Do(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got status %d; want %d", res.StatusCode, http.StatusOK)
	}

	requests := fakeClient.Requests()
	if len(requests) != 2 {
		t.Fatalf("got %d requests; want %d", len(requests), 2)
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

	// This is not deterministic, but it is enough... I think :-)
	time.Sleep(2 * timeoutPerRequest)
	if firstReq.Context().Err() == nil {
		t.Fatalf("expected first request to have expired context")
	}
	if secondReq.Context().Err() == nil {
		t.Fatalf("expected second request to have expired context")
	}
}

func TestRetrierExponentialBackoff(t *testing.T) {
	fakeClient := NewFakeClient()
	gotSleepPeriods := []time.Duration{}
	gotContexts := []context.Context{}
	sleep := func(ctx context.Context, period time.Duration) {
		gotContexts = append(gotContexts, ctx)
		gotSleepPeriods = append(gotSleepPeriods, period)
	}

	client := xhttp.NewRetrierClient(fakeClient,
		xhttp.RetrierWithMinSleepPeriod(time.Second),
		xhttp.RetrierWithSleep(sleep),
	)

	wantSleepPeriods := []time.Duration{
		time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
	}

	for range 4 {
		fakeClient.PushResponse(&http.Response{
			StatusCode: http.StatusServiceUnavailable,
		})
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

	assertEqual(t, gotSleepPeriods, wantSleepPeriods)
	for i, gotContext := range gotContexts {
		if gotContext != ctx {
			t.Errorf("got ctx[%d] %v != want %v", i, gotContext, ctx)
		}
	}
}

func TestRetrierRetryStatusCodes(t *testing.T) {
	t.Skip("TODO: rewriter with simpler approach")

	retryStatusCodes := []int{
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
	}
	for _, wantStatus := range retryStatusCodes {
		for _, wantMethod := range httpMethods() {

			t.Run(fmt.Sprintf("%s %d", wantMethod, wantStatus), func(t *testing.T) {
				server := NewServer()
				defer server.Close()

				client := xhttp.NewRetrierClient(&http.Client{}, noSleep())
				wantPath := "/" + t.Name()
				retryDone := make(chan struct{})

				go func() {
					assertReq := func(req Request) {
						if req.URL == nil {
							t.Errorf("pending request has no URL")
							return
						}
						if req.URL.Path != wantPath {
							t.Errorf("got path %q; want %q", req.URL.Path, wantPath)
						}
						if req.Method != wantMethod {
							t.Errorf("got method %q; want %q", req.Method, wantMethod)
						}
					}
					req := <-server.Requests()
					assertReq(req)
					req.SendResponse(Response{
						Status: wantStatus,
					})

					retryReq := <-server.Requests()
					assertReq(retryReq)
					retryReq.SendResponse(Response{
						Status: http.StatusOK,
					})
					close(retryDone)
				}()

				url := server.URL + wantPath
				request := newRequest(t, wantMethod, url, nil)

				res, err := client.Do(request)
				<-retryDone

				if err != nil {
					t.Fatalf("client.Do(%v) failed: %v", request, err)
				}
				if res.StatusCode != http.StatusOK {
					t.Fatalf("got status %v; want %v", res.StatusCode, http.StatusOK)
				}

				assertNoPendingRequests(t, server)
			})
		}
	}
}

func TestRetrierNoRetryStatusCodes(t *testing.T) {
	for wantStatus := 200; wantStatus < 500; wantStatus++ {
		if wantStatus == http.StatusTooManyRequests {
			continue
		}
		for _, wantMethod := range httpMethods() {

			t.Run(fmt.Sprintf("%s %d", wantMethod, wantStatus), func(t *testing.T) {
				fakeClient := NewFakeClient()
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

func assertNoPendingRequests(t *testing.T, s *Server) {
	t.Helper()

	select {
	case v := <-s.Requests():
		t.Fatalf("unexpected pending request: %v", v)
	default:
	}
}

func newRequest(t *testing.T, method, url string, body []byte) *http.Request {
	t.Helper()

	request, err := http.NewRequest(method, url, bytes.NewReader(body))
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
