// Package cli wires up the cobra command tree for the skillpack binary.
//
// Each command lives in its own file and shares the root state through
// package-level helpers. The CLI never calls os.Exit directly — the main
// entrypoint translates RunE errors into exit codes via exitcode.Classify.
package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
)

// Version is set by the linker via -ldflags="-X" at release time. Default for
// dev builds: "dev".
var Version = "dev"

// rootState holds shared CLI flags and io streams. We keep it global to
// avoid threading it through every command, but tests can inject fresh state
// via NewRoot.
type rootState struct {
	root    string
	verbose bool
	json    bool
	stdout  io.Writer
	stderr  io.Writer
}

// NewRoot constructs a fresh cobra command tree with its own io streams.
// Tests use this to capture output; the production main calls NewRoot and
// then Execute.
func NewRoot(stdout, stderr io.Writer) *cobra.Command {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	state := &rootState{stdout: stdout, stderr: stderr}
	cmd := &cobra.Command{
		Use:   "skillpack",
		Short: "Package manager and lockfile for agent skills",
		Long: `skillpack is a package manager, lockfile, and bundler for agent skills.

It reads SKILL.md, .cursorrules, AGENT.md, and skill.yaml files, resolves
semver dependencies between them, computes content-addressed sha256 hashes,
and produces deterministic skillpack.lock files and signed *.skl tarballs.

Single Go binary, zero runtime dependencies.`,
		Version:           Version,
		SilenceUsage:      true,
		SilenceErrors:     true,
		DisableAutoGenTag: true,
	}
	cmd.PersistentFlags().StringVarP(&state.root, "root", "r", ".", "workspace root directory")
	cmd.PersistentFlags().BoolVarP(&state.verbose, "verbose", "v", false, "verbose output")
	cmd.PersistentFlags().BoolVar(&state.json, "json", false, "emit JSON output where supported")

	cmd.AddCommand(newInitCmd(state))
	cmd.AddCommand(newAddCmd(state))
	cmd.AddCommand(newResolveCmd(state))
	cmd.AddCommand(newInstallCmd(state))
	cmd.AddCommand(newVerifyCmd(state))
	cmd.AddCommand(newBundleCmd(state))
	cmd.AddCommand(newSignCmd(state))
	cmd.AddCommand(newKeygenCmd(state))
	cmd.AddCommand(newLockCmd(state))

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	return cmd
}

// Execute runs the cobra root and returns an exit code suitable for os.Exit.
//
// Error -> exit code mapping:
//
//	nil error              -> 0
//	exitcode-wrapped error -> the wrapped class
//	any other error        -> 4 (Internal)
//	cobra usage error      -> 5 (Usage)
func Execute(stdout, stderr io.Writer, args []string) int {
	cmd := NewRoot(stdout, stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	if err == nil {
		return exitcode.OK
	}
	// Try to classify by wrapped exitcode first.
	if code := exitcode.Classify(err); code != exitcode.Internal {
		fmt.Fprintln(stderr, "error:", err)
		return code
	}
	// Cobra errors land here as plain errors. Treat them as usage errors.
	fmt.Fprintln(stderr, "error:", err)
	return exitcode.Usage
}
