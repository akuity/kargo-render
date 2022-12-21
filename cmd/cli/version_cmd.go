package cli

import (
	"github.com/spf13/cobra"

	"github.com/akuityio/bookkeeper/internal/version"
)

func newVersionCommand() *cobra.Command {
	const desc = "Print version information"
	cmd := &cobra.Command{
		Use:   "version",
		Short: desc,
		Long:  desc,
		RunE:  runVersionCommand,
	}
	cmd.Flags().AddFlagSet(flagSetOutput)
	return cmd
}

func runVersionCommand(cmd *cobra.Command, args []string) error {
	clientVersion := version.GetVersion()

	outputFormat, err := cmd.Flags().GetString(flagOutput)
	if err != nil {
		return err
	}
	if outputFormat == "" {
		outputFormat = flagOutputJSON
	}

	return output(clientVersion, cmd.OutOrStdout(), outputFormat)
}
