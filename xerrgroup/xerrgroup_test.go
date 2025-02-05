package xerrgroup_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/birdie-ai/golibs/xerrgroup"
	"github.com/google/go-cmp/cmp"
)

func TestCollectEmpty(t *testing.T) {
	g := xerrgroup.New[string]()
	got, err := g.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) > 0 {
		t.Fatalf("got %v; want empty", got)
	}
}

func TestCollectOne(t *testing.T) {
	want := t.Name()
	g := xerrgroup.New[string]()

	g.Go(func() (string, error) {
		return want, nil
	})

	got, err := g.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(got, []string{want}); diff != "" {
		t.Fatal(diff)
	}
}

func TestCollectN(t *testing.T) {
	want := []string{"a", "b", "c"}
	g := xerrgroup.New[string]()

	g.Go(func() (string, error) {
		return want[0], nil
	})
	g.Go(func() (string, error) {
		return want[1], nil
	})
	g.Go(func() (string, error) {
		return want[2], nil
	})

	got, err := g.Wait()
	if err != nil {
		t.Fatal(err)
	}

	slices.Sort(got)
	if diff := cmp.Diff(got, want); diff != "" {
		t.Fatal(diff)
	}
}

func TestPartialFailure(t *testing.T) {
	want := []string{"a", "b"}
	wantErr := errors.New("err")
	w := &sync.WaitGroup{}
	g := xerrgroup.New[string]()

	w.Add(2)
	g.Go(func() (string, error) {
		w.Done()
		return want[0], nil
	})
	g.Go(func() (string, error) {
		w.Done()
		return want[1], nil
	})
	g.Go(func() (string, error) {
		// Guarantee that previous subtasks have started already
		w.Wait()
		return "", wantErr
	})

	got, err := g.Wait()
	if !errors.Is(err, wantErr) {
		t.Fatalf("got err %v; want: %v", err, wantErr)
	}

	slices.Sort(got)
	if diff := cmp.Diff(got, want); diff != "" {
		t.Fatal(diff)
	}
}

func TestWithContext(t *testing.T) {
	causeErr := errors.New("fail")
	g, ctx := xerrgroup.WithContext[string](context.Background())

	g.Go(func() (string, error) {
		return "", causeErr
	})

	// Since we are not calling Wait yet this can only work if cancellation worked properly
	<-ctx.Done()
	gotErr := context.Cause(ctx)
	if !errors.Is(gotErr, causeErr) {
		t.Fatalf("got err %v; want %v", gotErr, causeErr)
	}
}
