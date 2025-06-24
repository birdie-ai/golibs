package xerrors_test

import (
	"errors"
	"testing"

	"github.com/birdie-ai/golibs/xerrors"
)

func TestTagIs(t *testing.T) {
	errTag := errors.New("error tag")
	origErr := errors.New("original error")
	taggedErr := xerrors.Tag(origErr, errTag)

	if !errors.Is(taggedErr, origErr) {
		t.Fatal("tagged err doesn't match the original error")
	}
	if !errors.Is(taggedErr, errTag) {
		t.Fatal("tagged err doesn't match the tag")
	}
	if taggedErr.Error() != origErr.Error() {
		t.Fatalf("tagged err msg %q != original error %q", taggedErr.Error(), origErr.Error())
	}
}

func TestTagAs(t *testing.T) {
	errTag := customError{"custom error"}
	origErr := customError2{"original error"}
	taggedErr := xerrors.Tag(origErr, errTag)

	if taggedErr.Error() != origErr.Error() {
		t.Fatalf("tagged err msg %q != original error %q", taggedErr.Error(), origErr.Error())
	}

	var gotTag customError
	if !errors.As(taggedErr, &gotTag) {
		t.Fatal("unable to retrieve error tag")
	}
	if gotTag != errTag {
		t.Fatalf("got %v; want %v", gotTag, errTag)
	}

	var gotOrig customError2
	if !errors.As(taggedErr, &gotOrig) {
		t.Fatal("unable to retrieve error tag")
	}
	if gotOrig != origErr {
		t.Fatalf("got %v; want %v", gotOrig, origErr)
	}
}

type (
	customError struct {
		msg string
	}
	customError2 struct {
		msg string
	}
)

func (c customError) Error() string {
	return c.msg
}

func (c customError2) Error() string {
	return c.msg
}
