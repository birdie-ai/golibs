package xhttp_test

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/birdie-ai/golibs/xhttp"
)

func TestRetrierNoRetryStatusCodes(t *testing.T) {
	httpMethods := []string{
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

	for wantStatus := 200; wantStatus < 500; wantStatus++ {
		for _, wantMethod := range httpMethods {
			t.Run(fmt.Sprintf("%s %d", wantMethod, wantStatus), func(t *testing.T) {
				server := NewServer()
				client := xhttp.NewRetrierClient(&http.Client{})
				wantPath := "/" + t.Name()
				wantBody := t.Name()

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
				request := newRequest(t, wantMethod, url, wantBody)

				res, err := client.Do(request)
				if err != nil {
					t.Fatalf("client.Do(%v) failed: %v", request, err)
				}
				if res.StatusCode != wantStatus {
					t.Fatalf("got status %v; want %v", res.StatusCode, wantStatus)
				}
			})
		}
	}
}

func newRequest(t *testing.T, method, url string, body string) *http.Request {
	t.Helper()

	request, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func readAll(t *testing.T, r io.Reader) []byte {
	t.Helper()

	v, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
