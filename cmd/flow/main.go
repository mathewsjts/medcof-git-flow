// Command flow propaga commits de uma feature (um único PR para main) para as
// branches de ambiente (dev/hom) por cherry-pick, com rastreabilidade e um
// fluxo de conflito que nunca resolve sozinho.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mathewsjts/medcof-git-flow/internal/flow"
	"github.com/mathewsjts/medcof-git-flow/internal/git"
	"github.com/mathewsjts/medcof-git-flow/internal/github"
	"github.com/mathewsjts/medcof-git-flow/internal/ui"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		// Cobra já imprime o erro; sair com código != 0.
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "flow",
		Short:         "Propaga commits de um PR para as branches de ambiente via cherry-pick",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.AddCommand(newSetupCmd())
	root.AddCommand(newDevCmd())
	return root
}

// bootstrap resolve o App para o repo corrente, com saída no stdout do comando.
func bootstrap(cmd *cobra.Command) (*flow.App, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return flow.Bootstrap(wd, flow.Deps{Out: cmd.OutOrStdout()})
}

func newDevCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dev",
		Short: "Aplica os commits do PR da branch atual no ambiente dev",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := bootstrap(cmd)
			if err != nil {
				return err
			}
			return app.Dev()
		},
	}
}

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Checa pré-requisitos (git, gh) e ativa rerere no repo local",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			g := git.New(wd)
			gh := github.NewCLI(wd)
			p := ui.New(cmd.OutOrStdout())
			if err := flow.Setup(g, gh, p); err != nil {
				return fmt.Errorf("setup não concluído")
			}
			return nil
		},
	}
}
