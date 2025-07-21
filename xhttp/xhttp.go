package xhttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type (
	// Response is an extension of [http.Response] that contains parsed data
	// instead of a response body.
	Response[T any] struct {
		*http.Response
		// RawBody is the raw response from where Value was parsed (useful mostly for debugging).
		RawBody []byte
		// Value is the parsed JSON response.
		Value T
		// Elapsed is how long it took for the request to be answered.
		Elapsed time.Duration
	}
	// ResponseParseErr is the error returned by [Do] if parsing the response body fails.
	ResponseParseErr struct {
		Err        error
		Body       []byte
		StatusCode int
		// Elapsed is how long it took for the request to be answered.
		Elapsed time.Duration
	}
)

// Do calls [Client.Do] and unmarshalls the HTTP response as a JSON of type [T].
// The returned [Response] embeds the original [http.Response] with the addition
// of the [Response.Obj] field that holds the parsed response.
//
// The original [http.Response.Body] will always be read and closed, the caller should ignore
// this field and use [Response.Value] to access the parsed response or use errors.As
// to check details in the case of an error (eg. debugging malformed JSON).
//
// If the response is not valid JSON an error of type [ResponseParseErr] is returned.
func Do[T any](c Client, req *http.Request) (*Response[T], error) {
	start := time.Now()
	v, err := c.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(v.Body)
	if err != nil {
		return nil, errors.Join(err, v.Body.Close())
	}
	if err := v.Body.Close(); err != nil {
		return nil, fmt.Errorf("xhttp: closing response body: %w", err)
	}

	var parsed T
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, ResponseParseErr{Err: err, Body: body, StatusCode: v.StatusCode, Elapsed: elapsed}
	}
	return &Response[T]{Response: v, RawBody: body, Value: parsed, Elapsed: elapsed}, nil
}

// Get is a helper that creates a HTTP request with a GET method and no request body and calls [Do].
// It will behave exactly as documented on [Do].
func Get[T any](ctx context.Context, c Client, url string) (*Response[T], error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return Do[T](c, r)
}

func (r ResponseParseErr) Error() string {
	return r.Err.Error()
}
