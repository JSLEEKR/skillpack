// Package exitcode defines the canonical CLI exit codes used across skillpack.
//
// CI pipelines rely on distinct exit codes to distinguish "the skills drifted"
// (expected failure, triggers alerts) from "the tool itself failed" (config
// error, needs human).
package exitcode

import "errors"

// Canonical exit codes.
const (
	OK       = 0 // success
	Drift    = 1 // hash or version drift detected
	Parse    = 2 // malformed input (YAML, frontmatter, manifest)
	IO       = 3 // filesystem / permission error
	Internal = 4 // unexpected bug; should not happen in practice
	Usage    = 5 // CLI usage error (bad flags)
)

// Class is a typed wrapper so errors can carry an exit code through
// errors.Is / errors.As.
type Class int

// Error returns a human label for the class.
func (c Class) Error() string {
	switch int(c) {
	case OK:
		return "ok"
	case Drift:
		return "drift"
	case Parse:
		return "parse error"
	case IO:
		return "io error"
	case Internal:
		return "internal error"
	case Usage:
		return "usage error"
	default:
		return "unknown"
	}
}

// Wrap attaches a Class to any error so that Classify can retrieve it later.
// A nil input returns nil (propagating success).
func Wrap(class int, err error) error {
	if err == nil {
		return nil
	}
	return &wrapped{class: Class(class), err: err}
}

type wrapped struct {
	class Class
	err   error
}

func (w *wrapped) Error() string { return w.err.Error() }
func (w *wrapped) Unwrap() error { return w.err }
func (w *wrapped) Is(target error) bool {
	var c Class
	if errors.As(target, &c) {
		return w.class == c
	}
	return false
}

// Classify inspects err and returns the exit code to use.
// Plain errors default to Internal so no error escapes without an exit code.
func Classify(err error) int {
	if err == nil {
		return OK
	}
	var w *wrapped
	if errors.As(err, &w) {
		return int(w.class)
	}
	return Internal
}
