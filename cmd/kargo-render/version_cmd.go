package main

import (
	"context"
	"io"

	"github.com/spf13/cobra"

	"github.com/akuity/kargo-render/internal/version"
)

type versionOptions struct {
	outputFormat string
}

func newVersionCommand() *cobra.Command {
	cmdOpts := &versionOptions{}

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmdOpts.run(cmd.Context(), cmd.OutOrStdout())
		},
	}

	// Register the option flags on the command.
	cmdOpts.addFlags(cmd)

	return cmd
}

// addFlags adds the flags for the version options to the provided command.
func (o *versionOptions) addFlags(cmd *cobra.Command) {
	const flagOutput = "output"
	cmd.Flags().StringVarP(
		&o.outputFormat,
		flagOutput,
		"o",
		"",
		"Specify a format for command output (json or yaml).",
	)
}

// run prints version information.
func (o *versionOptions) run(_ context.Context, out io.Writer) error {
	if o.outputFormat == "" {
		o.outputFormat = "json"
	}
	return output(version.GetVersion(), out, o.outputFormat)
}
