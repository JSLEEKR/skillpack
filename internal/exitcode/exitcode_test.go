package exitcode

import (
	"errors"
	"fmt"
	"testing"
)

func TestClassifyNil(t *testing.T) {
	if got := Classify(nil); got != OK {
		t.Errorf("Classify(nil) = %d, want %d", got, OK)
	}
}

func TestClassifyPlainError(t *testing.T) {
	if got := Classify(errors.New("boom")); got != Internal {
		t.Errorf("Classify(plain) = %d, want %d", got, Internal)
	}
}

func TestClassifyWrapped(t *testing.T) {
	tests := []int{Drift, Parse, IO, Internal, Usage}
	for _, c := range tests {
		err := Wrap(c, errors.New("x"))
		if got := Classify(err); got != c {
			t.Errorf("Classify(Wrap(%d)) = %d, want %d", c, got, c)
		}
	}
}

func TestWrapNil(t *testing.T) {
	if Wrap(Drift, nil) != nil {
		t.Errorf("Wrap(_, nil) should be nil")
	}
}

func TestWrapPreservesMessage(t *testing.T) {
	inner := errors.New("the inner reason")
	wrapped := Wrap(Drift, inner)
	if wrapped.Error() != "the inner reason" {
		t.Errorf("Error() = %q, want %q", wrapped.Error(), "the inner reason")
	}
}

func TestWrapUnwrap(t *testing.T) {
	inner := errors.New("inner")
	wrapped := Wrap(Drift, inner)
	if !errors.Is(wrapped, inner) {
		t.Errorf("errors.Is should find inner")
	}
}

func TestWrapDoubleWrap(t *testing.T) {
	inner := errors.New("inner")
	w1 := Wrap(Drift, inner)
	w2 := fmt.Errorf("context: %w", w1)
	if got := Classify(w2); got != Drift {
		t.Errorf("Classify(double wrap) = %d, want %d", got, Drift)
	}
}

func TestClassError(t *testing.T) {
	cases := map[int]string{
		OK:       "ok",
		Drift:    "drift",
		Parse:    "parse error",
		IO:       "io error",
		Internal: "internal error",
		Usage:    "usage error",
		99:       "unknown",
	}
	for c, want := range cases {
		if got := Class(c).Error(); got != want {
			t.Errorf("Class(%d).Error() = %q, want %q", c, got, want)
		}
	}
}
