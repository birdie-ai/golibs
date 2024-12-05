package xhttp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
)

var defaultUserAgent string

func init() {
	bf, ok := debug.ReadBuildInfo()
	if !ok {
		defaultUserAgent = "golibs-xhttp-client/0"
		return
	}
	ppath := strings.Split(bf.Path, "/")
	name := ppath[len(ppath)-1]
	defaultUserAgent = fmt.Sprintf("Go/%s %s/%s", bf.GoVersion, name, bf.Main.Version)
}

// NewRequestWithContext is a wrapper that will call [http.NewRequestWithContext] and add an User-Agent header according to [RFC 7231].
// It is a more complete User-Agent than Go's default, including proper Go version and the name of the main package of the binary.
// The user agent will be on the format: Go/<go version> <main package name>/<commit ID>
//
// [RFC 7231]: https://datatracker.ietf.org/doc/html/rfc7231#section-5.5.3
func NewRequestWithContext(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return req, err
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	return req, nil
}
