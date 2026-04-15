package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
	"github.com/JSLEEKR/skillpack/internal/signer"
)

func newSignCmd(state *rootState) *cobra.Command {
	var keyPath, outPath string
	var verifyMode bool
	var pubPath string
	cmd := &cobra.Command{
		Use:   "sign <bundle.skl>",
		Short: "Sign or verify a bundle with an ed25519 key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := args[0]
			if verifyMode {
				if pubPath == "" {
					return exitcode.Wrap(exitcode.Usage, fmt.Errorf("--pubkey required for --verify"))
				}
				sig := payload + ".sig"
				if outPath != "" {
					sig = outPath
				}
				if err := signer.VerifyFile(pubPath, payload, sig); err != nil {
					return err
				}
				fmt.Fprintf(state.stdout, "skillpack: signature OK for %s\n", payload)
				return nil
			}
			if keyPath == "" {
				return exitcode.Wrap(exitcode.Usage, fmt.Errorf("--key required to sign"))
			}
			out := outPath
			if out == "" {
				out = payload + ".sig"
			}
			if err := signer.SignFile(keyPath, payload, out); err != nil {
				return err
			}
			fmt.Fprintf(state.stdout, "skillpack: wrote signature %s\n", out)
			return nil
		},
	}
	cmd.Flags().StringVar(&keyPath, "key", "", "private key file (ed25519)")
	cmd.Flags().StringVar(&pubPath, "pubkey", "", "public key file (ed25519, --verify mode)")
	cmd.Flags().StringVarP(&outPath, "out", "o", "", "signature output path (default: <bundle>.sig)")
	cmd.Flags().BoolVar(&verifyMode, "verify", false, "verify mode (requires --pubkey)")
	return cmd
}

func newKeygenCmd(state *rootState) *cobra.Command {
	var privPath, pubPath string
	cmd := &cobra.Command{
		Use:   "keygen",
		Short: "Generate an ed25519 keypair for signing bundles",
		RunE: func(cmd *cobra.Command, args []string) error {
			if privPath == "" || pubPath == "" {
				return exitcode.Wrap(exitcode.Usage, fmt.Errorf("--priv and --pub are required"))
			}
			priv, pub, err := signer.GenerateKeypair()
			if err != nil {
				return err
			}
			if err := writeFileSecure(privPath, priv); err != nil {
				return err
			}
			if err := writeFileSecure(pubPath, pub); err != nil {
				return err
			}
			fmt.Fprintf(state.stdout, "skillpack: wrote keypair (%s, %s)\n", privPath, pubPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&privPath, "priv", "", "private key output path")
	cmd.Flags().StringVar(&pubPath, "pub", "", "public key output path")
	return cmd
}
