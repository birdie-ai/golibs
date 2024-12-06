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
	want := "Go/" + bf.GoVersion

	if want != v.UserAgent() {
		t.Fatalf("got user agent %q; want %q", v.UserAgent(), want)
	}
}
