package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
	"github.com/JSLEEKR/skillpack/internal/hasher"
	"github.com/JSLEEKR/skillpack/internal/lockfile"
	"github.com/JSLEEKR/skillpack/internal/workspace"
)

func newLockCmd(state *rootState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Rewrite skillpack.lock from current workspace state",
		Long: `Equivalent to 'install' but always overwrites skillpack.lock without
running 'install' side effects (does not create the file if missing — use
'install' for first-time generation).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			abs, err := filepath.Abs(state.root)
			if err != nil {
				return exitcode.Wrap(exitcode.IO, err)
			}
			path := filepath.Join(abs, "skillpack.lock")
			loaded, err := workspace.Load(state.root)
			if err != nil {
				return err
			}
			for _, s := range loaded.Skills {
				s.Hash = hasher.Hash(s)
			}
			lf := lockfile.FromSkills(loaded.Skills)
			if err := lockfile.WriteFile(path, lf); err != nil {
				return err
			}
			fmt.Fprintf(state.stdout, "skillpack: rewrote %s\n", path)
			return nil
		},
	}
	return cmd
}

// pluralSkill returns "skill" or "skills" depending on count, so CLI messages
// read correctly for n == 1 ("wrote lock (1 skill)") vs the plural case.
func pluralSkill(n int) string {
	if n == 1 {
		return "skill"
	}
	return "skills"
}

// writeFileSecure writes data to path with mode 0600, creating parent dirs as
// needed. Used by `keygen` so private keys are not world-readable.
func writeFileSecure(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return exitcode.Wrap(exitcode.IO, fmt.Errorf("mkdir: %w", err))
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return exitcode.Wrap(exitcode.IO, fmt.Errorf("write %s: %w", path, err))
	}
	return nil
}
