package xerrgroup_test

import (
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/birdie-ai/golibs/xerrgroup"
	"github.com/google/go-cmp/cmp"
)

func TestCollectEmpty(t *testing.T) {
	g := &xerrgroup.Group[string]{}
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
	g := &xerrgroup.Group[string]{}

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
	g := &xerrgroup.Group[string]{}

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
	g := &xerrgroup.Group[string]{}

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
		// Guarantee that returning error here won't stop other subtasks from returning values
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
