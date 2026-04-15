package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
	"github.com/JSLEEKR/skillpack/internal/workspace"
)

func newResolveCmd(state *rootState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve skill dependencies and print install order",
		Long:  `Reads the workspace manifest and prints the topological install order. Does not write any files.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := workspace.Load(state.root)
			if err != nil {
				return err
			}
			if state.json {
				type entry struct {
					Name    string `json:"name"`
					Version string `json:"version"`
					Format  string `json:"format"`
					Source  string `json:"source"`
				}
				out := make([]entry, 0, len(loaded.Skills))
				for _, s := range loaded.Skills {
					out = append(out, entry{
						Name: s.Name, Version: s.Version, Format: string(s.Format), Source: s.SourcePath,
					})
				}
				enc := json.NewEncoder(state.stdout)
				enc.SetIndent("", "  ")
				if err := enc.Encode(out); err != nil {
					return exitcode.Wrap(exitcode.Internal, err)
				}
				return nil
			}
			if len(loaded.Skills) == 0 {
				fmt.Fprintln(state.stdout, "skillpack: no skills found")
				return nil
			}
			fmt.Fprintf(state.stdout, "Install order (%d skills):\n", len(loaded.Skills))
			for i, s := range loaded.Skills {
				fmt.Fprintf(state.stdout, "  %d. %s@%s [%s] — %s\n", i+1, s.Name, s.Version, s.Format, s.SourcePath)
			}
			return nil
		},
	}
	return cmd
}
