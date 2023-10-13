package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var desc = "Kargo Render renders environment-specific manifests into " +
	"environment-specific branches of your gitops repos"

func newRootCommand() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:              "kargo-render",
		Short:            desc,
		Long:             desc,
		PersistentPreRun: persistentPreRun,
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
		DisableAutoGenTag: true,
		SilenceUsage:      true,
	}
	renderCommand, err := newRenderCommand()
	if err != nil {
		return nil, err
	}
	command.AddCommand(renderCommand)
	command.AddCommand(newVersionCommand())
	return command, nil
}

func persistentPreRun(cmd *cobra.Command, _ []string) {
	cmd.Flags().VisitAll(
		func(flag *pflag.Flag) {
			switch flag.Name {
			case flagRepoPassword, flagRepoUsername:
				if !flag.Changed {
					envVarName := fmt.Sprintf(
						"KARGO_RENDER_%s",
						strings.ReplaceAll(
							strings.ToUpper(flag.Name),
							"-",
							"_",
						),
					)
					envVarValue := os.Getenv(envVarName)
					if envVarValue != "" {
						if err := cmd.Flags().Set(flag.Name, envVarValue); err != nil {
							fmt.Println(err)
							os.Exit(1)
						}
					}
				}
			}
		},
	)
}
