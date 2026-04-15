package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
	"github.com/JSLEEKR/skillpack/internal/lockfile"
	"github.com/JSLEEKR/skillpack/internal/manifest"
	"github.com/JSLEEKR/skillpack/internal/verify"
	"github.com/JSLEEKR/skillpack/internal/workspace"
)

func newVerifyCmd(state *rootState) *cobra.Command {
	var lockPath string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "CI: exit non-zero if installed skills drift from the lockfile",
		Long: `Reads skillpack.lock and every skill file in the workspace, recomputes
the sha256 hash of each skill, and compares with the lockfile.

Exit codes:
  0  all skills match the lockfile
  1  hash or version drift detected
  2  parse error in a skill file
  3  I/O error
  4  internal error
  6  security (signature verification failed — sign subcommand only)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			abs, err := filepath.Abs(state.root)
			if err != nil {
				return exitcode.Wrap(exitcode.IO, err)
			}
			lp := lockPath
			if lp == "" {
				lp = filepath.Join(abs, "skillpack.lock")
			}
			lf, err := lockfile.ReadFile(lp)
			if err != nil {
				return err
			}
			// Discover skill files WITHOUT running the resolver. A missing or
			// deleted skill file is the textbook "drift" case: verify.Run will
			// produce `missing` findings that map to exit code 1 via
			// Result.ExitCode(). Running the full resolver here would wrap
			// missing-dep errors as Parse errors (exit 2), conflating drift
			// with malformed config and breaking CI branching.
			manPath := filepath.Join(abs, "skillpack.yaml")
			w, err := manifest.ReadFile(manPath)
			if err != nil {
				// Fall back to skillpack.yml.
				alt := filepath.Join(abs, "skillpack.yml")
				w, err = manifest.ReadFile(alt)
				if err != nil {
					return err
				}
			}
			files, err := workspace.Discover(abs, w.Skills)
			if err != nil {
				return err
			}
			res, err := verify.Run(lf, files)
			if err != nil {
				return err
			}
			if state.json {
				enc := json.NewEncoder(state.stdout)
				enc.SetIndent("", "  ")
				if err := enc.Encode(res); err != nil {
					return exitcode.Wrap(exitcode.Internal, err)
				}
			} else {
				fmt.Fprintln(state.stdout, res.Summary())
				for _, f := range res.Findings {
					fmt.Fprintf(state.stdout, "  [%s] %s: %s\n", f.Kind, f.Name, f.Message)
				}
			}
			if !res.OK {
				return exitcode.Wrap(exitcode.Drift, fmt.Errorf("drift detected"))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&lockPath, "lockfile", "", "path to skillpack.lock (default: <root>/skillpack.lock)")
	return cmd
}
