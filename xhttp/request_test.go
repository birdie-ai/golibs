package xhttp_test

import (
	"context"
	"net/http"
	"runtime/debug"
	"testing"

	"github.com/birdie-ai/golibs/xhttp"
)

func TestRequestUserAgent(t *testing.T) {
	v, err := xhttp.NewRequestWithContext(context.Background(), http.MethodPost, "http://test", nil)
	if err != nil {
		t.Fatal(err)
	}
	bf, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("test supposed to have build information")
	}
	// The test only has the Go version, no main build info available
	want := "xhttp.test/no-version Go/" + bf.GoVersion

	if want != v.UserAgent() {
		t.Fatalf("got user agent %q; want %q", v.UserAgent(), want)
	}
}

func TestJSONRequestContentType(t *testing.T) {
	v, err := xhttp.NewJSONRequest(context.Background(), http.MethodPost, "http://test", map[string]string{"key": "value"})
	if err != nil {
		t.Fatal(err)
	}
	want := "application/json"
	got := v.Header.Get("Content-Type")
	if want != got {
		t.Fatalf("got content type %q; want %q", got, want)
	}
}
