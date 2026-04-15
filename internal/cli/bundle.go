package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/JSLEEKR/skillpack/internal/bundle"
	"github.com/JSLEEKR/skillpack/internal/exitcode"
	"github.com/JSLEEKR/skillpack/internal/hasher"
	"github.com/JSLEEKR/skillpack/internal/lockfile"
	"github.com/JSLEEKR/skillpack/internal/workspace"
)

func newBundleCmd(state *rootState) *cobra.Command {
	var outPath string
	var listMode bool
	cmd := &cobra.Command{
		Use:   "bundle",
		Short: "Build a deterministic *.skl tarball from the workspace",
		Long: `Resolves the workspace, hashes every skill, embeds the manifest, and
writes a deterministic gzip-compressed tarball.

Two runs over the same input produce byte-identical archives, so the tarball
hash is itself a stable content address.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := workspace.Load(state.root)
			if err != nil {
				return err
			}
			for _, s := range loaded.Skills {
				s.Hash = hasher.Hash(s)
			}
			lf := lockfile.FromSkills(loaded.Skills)
			data, err := bundle.Bundle(loaded.Skills, lf)
			if err != nil {
				return err
			}
			if listMode {
				lines, err := bundle.Inspect(data)
				if err != nil {
					return err
				}
				for _, l := range lines {
					fmt.Fprintln(state.stdout, l)
				}
				return nil
			}
			abs, err := filepath.Abs(state.root)
			if err != nil {
				return exitcode.Wrap(exitcode.IO, err)
			}
			out := outPath
			if out == "" {
				out = filepath.Join(abs, loaded.Manifest.Name+".skl")
			}
			if err := bundle.WriteFile(out, data); err != nil {
				return err
			}
			fmt.Fprintf(state.stdout, "skillpack: wrote %s (%d bytes, %d skills)\n", out, len(data), len(loaded.Skills))
			fmt.Fprintf(state.stdout, "  hash: %s\n", hasher.HashBytes(data))
			return nil
		},
	}
	cmd.Flags().StringVarP(&outPath, "out", "o", "", "output path (default: <name>.skl in workspace root)")
	cmd.Flags().BoolVar(&listMode, "list", false, "list contents instead of writing")
	return cmd
}
