package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
	"github.com/JSLEEKR/skillpack/internal/hasher"
	"github.com/JSLEEKR/skillpack/internal/lockfile"
	"github.com/JSLEEKR/skillpack/internal/workspace"
)

func newInstallCmd(state *rootState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Resolve and write skillpack.lock",
		Long: `Reads the workspace, resolves dependencies, computes content-addressed
hashes for every skill, and writes a deterministic skillpack.lock.

Run 'skillpack verify' afterwards to check the lockfile against disk.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := workspace.Load(state.root)
			if err != nil {
				return err
			}
			// Compute hashes.
			for _, s := range loaded.Skills {
				s.Hash = hasher.Hash(s)
			}
			lf := lockfile.FromSkills(loaded.Skills)
			abs, err := filepath.Abs(state.root)
			if err != nil {
				return exitcode.Wrap(exitcode.IO, err)
			}
			path := filepath.Join(abs, "skillpack.lock")
			if err := lockfile.WriteFile(path, lf); err != nil {
				return err
			}
			fmt.Fprintf(state.stdout, "skillpack: wrote %s (%d skills)\n", path, len(lf.Skills))
			if state.verbose {
				for _, e := range lf.Skills {
					fmt.Fprintf(state.stdout, "  %s\n", lockfile.FormatHashLine(e))
				}
			}
			return nil
		},
	}
	return cmd
}
