package xhttp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/birdie-ai/golibs/xhttp"
)

func TestDo(t *testing.T) {
	type (
		Error struct {
			Message string
		}
		OK struct {
			Success string
		}
		Response struct {
			OK
			Error Error
		}
	)

	wantOK := Response{OK: OK{Success: "success !!!"}}
	wantErr := Response{Error: Error{Message: "such error !!!"}}
	var sendErr bool

	server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
		if sendErr {
			res.WriteHeader(http.StatusInternalServerError)
			e := json.NewEncoder(res)
			if err := e.Encode(wantErr); err != nil {
				t.Error(err)
			}
			return
		}
		e := json.NewEncoder(res)
		if err := e.Encode(wantOK); err != nil {
			t.Error(err)
		}
	}))

	c := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := xhttp.Do[Response](c, req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got status code %d; want %d", res.StatusCode, http.StatusOK)
	}
	if res.Obj != wantOK {
		t.Fatalf("got response %v; want %v", res.Obj, wantOK)
	}

	sendErr = true
	res, err = xhttp.Do[Response](c, req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("got status code %d; want %d", res.StatusCode, http.StatusInternalServerError)
	}
	if res.Obj != wantErr {
		t.Fatalf("got response %v; want %v", res.Obj, wantErr)
	}
}
