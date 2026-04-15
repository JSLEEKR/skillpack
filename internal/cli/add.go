package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
	"github.com/JSLEEKR/skillpack/internal/manifest"
)

func newAddCmd(state *rootState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <path>",
		Short: "Add a skill source directory to the workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			abs, err := filepath.Abs(state.root)
			if err != nil {
				return exitcode.Wrap(exitcode.IO, err)
			}
			manPath := filepath.Join(abs, "skillpack.yaml")
			w, err := manifest.ReadFile(manPath)
			if err != nil {
				return err
			}
			added := w.AddSkillPath(args[0])
			if !added {
				fmt.Fprintf(state.stdout, "skillpack: %q already in workspace\n", args[0])
				return nil
			}
			if err := manifest.WriteFile(manPath, w); err != nil {
				return err
			}
			fmt.Fprintf(state.stdout, "skillpack: added %q to %s\n", args[0], manPath)
			return nil
		},
	}
	return cmd
}
