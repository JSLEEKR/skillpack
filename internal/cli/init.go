package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
	"github.com/JSLEEKR/skillpack/internal/manifest"
)

func newInitCmd(state *rootState) *cobra.Command {
	var name string
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a new skillpack workspace",
		Long: `Creates skillpack.yaml in the current root directory.

Default skill source path is ./skills. Use 'skillpack add' afterwards to add
more directories.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := state.root
			if root == "" {
				root = "."
			}
			abs, err := filepath.Abs(root)
			if err != nil {
				return exitcode.Wrap(exitcode.IO, err)
			}
			if err := os.MkdirAll(abs, 0755); err != nil {
				return exitcode.Wrap(exitcode.IO, err)
			}
			path := filepath.Join(abs, "skillpack.yaml")
			if _, err := os.Stat(path); err == nil && !force {
				return exitcode.Wrap(exitcode.Usage,
					fmt.Errorf("skillpack.yaml already exists at %s (use --force to overwrite)", path))
			}
			if name == "" {
				name = filepath.Base(abs)
				if name == "" || name == "." || name == "/" {
					name = "skillpack-workspace"
				}
			}
			w := manifest.Default(name)
			if err := manifest.WriteFile(path, w); err != nil {
				return err
			}
			// Ensure ./skills directory exists for convenience.
			_ = os.MkdirAll(filepath.Join(abs, "skills"), 0755)
			fmt.Fprintf(state.stdout, "skillpack: created %s\n", path)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "workspace name (defaults to directory name)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing skillpack.yaml")
	return cmd
}
