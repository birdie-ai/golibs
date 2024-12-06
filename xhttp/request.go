package xhttp

import (
	"context"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
)

var defaultUserAgent string

func init() {
	defaultUserAgent = "golibs-xhttp-client/0"

	bf, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	var uas []string

	ppath := strings.Split(bf.Path, "/")
	name := ppath[len(ppath)-1]

	if name != "" {
		version := "no-version"
		for _, buildSetting := range bf.Settings {
			if buildSetting.Key == "vcs.revision" {
				version = buildSetting.Value
				if len(version) > 7 {
					// Get a short sha of the version, useful for git
					version = version[0:7]
				}
			}
		}
		uas = append(uas, name+"/"+version)
	}

	if bf.GoVersion != "" {
		uas = append(uas, "Go/"+bf.GoVersion)
	}

	if len(uas) > 0 {
		defaultUserAgent = strings.Join(uas, " ")
	}
}

// NewRequestWithContext is a wrapper that will call [http.NewRequestWithContext] and add an User-Agent header according to [RFC 7231].
// It is a more complete User-Agent than Go's default, including proper Go version and the name of the main package of the binary with its version.
// The user agent will be on the format: Go/<go version> <main package name>/<main version>
// This is intended for internal communication between services since the user agent contains a lot of details about the client.
// If this seems like too much information to send to a public service just use [http.NewRequestWithContext].
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
