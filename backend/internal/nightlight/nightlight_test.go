package nightlight

import (
	"context"
	"errors"
	"testing"
)

type recorder struct {
	inhibits   int
	uninhibits []uint32
	inhibitErr error
	cookie     uint32
}

func (r *recorder) inhibitor() *Inhibitor {
	return newInhibitor(
		func(context.Context) (uint32, error) {
			r.inhibits++
			return r.cookie, r.inhibitErr
		},
		func(_ context.Context, cookie uint32) error {
			r.uninhibits = append(r.uninhibits, cookie)
			return nil
		},
	)
}

func TestSharedReasonsInhibitOnce(t *testing.T) {
	r := &recorder{cookie: 7}
	i := r.inhibitor()

	if err := i.Acquire(ReasonSync); err != nil {
		t.Fatal(err)
	}
	if err := i.Acquire(ReasonGaming); err != nil {
		t.Fatal(err)
	}
	if r.inhibits != 1 {
		t.Fatalf("inhibit called %d times, want 1", r.inhibits)
	}

	if err := i.Release(ReasonSync); err != nil {
		t.Fatal(err)
	}
	if len(r.uninhibits) != 0 {
		t.Fatalf("uninhibit called while gaming still held: %v", r.uninhibits)
	}

	if err := i.Release(ReasonGaming); err != nil {
		t.Fatal(err)
	}
	if len(r.uninhibits) != 1 || r.uninhibits[0] != 7 {
		t.Fatalf("uninhibit calls = %v, want [7]", r.uninhibits)
	}
}

func TestAcquireTwiceIsIdempotent(t *testing.T) {
	r := &recorder{cookie: 1}
	i := r.inhibitor()

	_ = i.Acquire(ReasonSync)
	_ = i.Acquire(ReasonSync)
	if r.inhibits != 1 {
		t.Fatalf("inhibit called %d times, want 1", r.inhibits)
	}

	_ = i.Release(ReasonSync)
	if len(r.uninhibits) != 1 {
		t.Fatalf("uninhibit calls = %v, want exactly one", r.uninhibits)
	}
}

func TestReleaseUnheldIsNoop(t *testing.T) {
	r := &recorder{cookie: 1}
	i := r.inhibitor()

	if err := i.Release(ReasonGaming); err != nil {
		t.Fatal(err)
	}
	if r.inhibits != 0 || len(r.uninhibits) != 0 {
		t.Fatalf("bus called for unheld release: inhibits=%d uninhibits=%v", r.inhibits, r.uninhibits)
	}
}

func TestInhibitFailureHoldsNothing(t *testing.T) {
	r := &recorder{inhibitErr: errors.New("no KWin")}
	i := r.inhibitor()

	if err := i.Acquire(ReasonSync); err == nil {
		t.Fatal("Acquire returned nil on inhibit failure")
	}
	r.inhibitErr = nil
	r.cookie = 3
	if err := i.Acquire(ReasonSync); err != nil {
		t.Fatal(err)
	}
	if r.inhibits != 2 {
		t.Fatalf("inhibit called %d times, want 2 (the failed attempt was not held)", r.inhibits)
	}
}

func TestReleaseAll(t *testing.T) {
	r := &recorder{cookie: 9}
	i := r.inhibitor()

	_ = i.Acquire(ReasonSync)
	_ = i.Acquire(ReasonGaming)
	if err := i.ReleaseAll(); err != nil {
		t.Fatal(err)
	}
	if len(r.uninhibits) != 1 || r.uninhibits[0] != 9 {
		t.Fatalf("uninhibit calls = %v, want [9]", r.uninhibits)
	}

	if err := i.ReleaseAll(); err != nil {
		t.Fatal(err)
	}
	if len(r.uninhibits) != 1 {
		t.Fatalf("second ReleaseAll called the bus again: %v", r.uninhibits)
	}
}

func TestFailedUninhibitKeepsReasonForRetry(t *testing.T) {
	r := &recorder{cookie: 5}
	i := r.inhibitor()
	_ = i.Acquire(ReasonSync)

	failUninhibit := true
	i.uninhibit = func(_ context.Context, cookie uint32) error {
		r.uninhibits = append(r.uninhibits, cookie)
		if failUninhibit {
			return errors.New("KWin timed out")
		}
		return nil
	}

	if err := i.Release(ReasonSync); err == nil {
		t.Fatal("Release returned nil on uninhibit failure")
	}
	failUninhibit = false
	if err := i.Release(ReasonSync); err != nil {
		t.Fatal(err)
	}
	if len(r.uninhibits) != 2 {
		t.Fatalf("uninhibit calls = %v, want a retry after the failure", r.uninhibits)
	}
}
