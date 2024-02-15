package xhttp_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type (
	// Response represents an http response injected on a test using a [Server] [Request].
	Response struct {
		Header http.Header
		Body   []byte
		Status int
	}

	// Request is an extension of [http.Request] used by [xhttp.Server] to inject responses.
	Request struct {
		*http.Request
		response chan Response
	}
	// Server is a wrapper for [httptest.Server] with a different approach to how to
	// inspect requests and inject responses.
	Server struct {
		*httptest.Server
		requests chan Request
		closed   bool
	}
)

// NewServer creates a new [Server]. Use [Server.Requests] to get requests and inject responses.
// [Server] aims to extend the [httptest.Server] providing an API that makes it easy to inspect
// arbitrary requests, block them for some time if necessary and inject whatever result you want.
// Blocking behavior is specially useful to test concurrent execution, you can easily check that you
// already received N concurrent requests on the server but leave the responses pending.
func NewServer() *Server {
	requests := make(chan Request)
	server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		responseChan := make(chan Response)
		requests <- Request{
			Request:  req,
			response: responseChan,
		}

		response := <-responseChan

		for key, vals := range response.Header {
			for _, val := range vals {
				res.Header().Add(key, val)
			}
		}
		res.WriteHeader(response.Status)
		_, _ = res.Write(response.Body)
	}))
	return &Server{server, requests, false}
}

// Close closes the underlying [Server].
// It is a programming error to use a [Request] after closing the associated [Server].
func (s *Server) Close() {
	if s.closed {
		return
	}
	close(s.requests)
	s.Server.Close()
}

// Requests is used to get pending requests receive by the server.
func (s *Server) Requests() <-chan Request {
	return s.requests
}

// SendResponse will send the given [Response] for this request.
// It is a programming error to call it more than once.
// It is a programming error to call it if the underlying [Server] is already closed.
func (r *Request) SendResponse(res Response) {
	r.response <- res
	close(r.response)
}

func TestServer(t *testing.T) {
	server := NewServer()
	client := &http.Client{}
	const (
		wantStatus = http.StatusOK
		wantMethod = http.MethodPost
		wantPath   = "/test"
		wantBody   = "test"
	)

	go func() {
		req := <-server.Requests()
		if req.URL.Path != wantPath {
			t.Errorf("got path %q; want %q", req.URL.Path, wantPath)
		}
		if req.Method != wantMethod {
			t.Errorf("got method %q; want %q", req.Method, wantMethod)
		}
		reqBody, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("reading request: %v", err)
		}
		req.SendResponse(Response{
			Status: wantStatus,
			Body:   reqBody,
		})
	}()

	url := server.URL + wantPath
	request := newRequest(t, wantMethod, url, []byte(wantBody))

	res, err := client.Do(request)
	if err != nil {
		t.Fatalf("client.Do(%v) failed: %v", request, err)
	}
	defer res.Body.Close() //nolint: errcheck
	if res.StatusCode != wantStatus {
		t.Fatalf("got status %v; want %v", res.StatusCode, wantStatus)
	}
	v, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	got := string(v)
	if got != wantBody {
		t.Fatalf("got response body %q; want %v", got, wantBody)
	}
}
