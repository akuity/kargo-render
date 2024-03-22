package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	render "github.com/akuity/kargo-render"
)

type rootOptions struct {
	*render.Request
	commitMessage string
	debug         bool
	outputFormat  string
}

func newRootCommand() *cobra.Command {
	cmdOpts := &rootOptions{
		Request: &render.Request{},
	}

	cmd := &cobra.Command{
		Use: "kargo-render",
		Short: "Render stage-specific manifests into a specific branch of " +
			"a remote gitops repo",
		DisableAutoGenTag: true,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		Args:   cobra.NoArgs,
		PreRun: cmdOpts.preRun,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmdOpts.run(cmd.Context(), cmd.OutOrStdout())
		},
	}

	// Register the option flags on the command.
	cmdOpts.addFlags(cmd)

	// Register the subcommands.
	cmd.AddCommand(newActionCommand())
	cmd.AddCommand(newVersionCommand())

	return cmd
}

// addFlags adds the flags for the root options to the provided command.
func (o *rootOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(
		&o.AllowEmpty,
		flagAllowEmpty,
		false,
		"Allow the rendered manifests to be empty. If not specified, this is "+
			"disallowed as a safeguard.",
	)

	cmd.Flags().StringVarP(
		&o.commitMessage,
		flagCommitMessage,
		"m",
		"",
		"A custom message to be used for the commit to the remote gitops repository.",
	)

	cmd.Flags().BoolVarP(
		&o.debug,
		flagDebug,
		"d",
		false,
		"Display debug output.",
	)

	cmd.Flags().StringArrayVarP(
		&o.Images,
		flagImage,
		"i",
		nil,
		"An image to be incorporated into the final result. This flag may be "+
			"used more than once.",
	)

	cmd.Flags().StringVar(
		&o.LocalInPath,
		flagLocalInPath,
		"",
		"Read input from the specified path instead of the remote gitops repository.",
	)

	cmd.Flags().StringVar(
		&o.LocalOutPath,
		flagLocalOutPath,
		"",
		"Write rendered manifests to the specified path instead of the remote "+
			"gitops repository. The path must NOT already exist.",
	)

	cmd.Flags().StringVarP(
		&o.outputFormat,
		flagOutput,
		"o",
		"",
		"Specify a format for command output (json or yaml).",
	)

	cmd.Flags().StringVarP(
		&o.Ref,
		flagRef,
		"R",
		"",
		"A branch or a precise commit in the remote gitops repository to use as "+
			"input. If this is not provided, Kargo Render renders from HEAD.",
	)

	cmd.Flags().StringVarP(
		&o.RepoURL,
		flagRepo,
		"r",
		"",
		"The URL of a remote gitops repository.",
	)

	cmd.Flags().StringVarP(
		&o.RepoCreds.Password,
		flagRepoPassword,
		"p",
		"",
		"Password or token for reading from and writing to the remote gitops "+
			"repository. Can alternatively be specified using the "+
			"KARGO_RENDER_REPO_PASSWORD environment variable.",
	)

	cmd.Flags().StringVarP(
		&o.RepoCreds.Username,
		flagRepoUsername,
		"u",
		"",
		"Username for reading from and writing to the remote gitops repository. "+
			"Can alternatively be specified using the KARGO_RENDER_REPO_USERNAME "+
			"environment variable.",
	)

	cmd.Flags().BoolVar(
		&o.Stdout,
		flagStdout,
		false,
		"Write rendered manifests to stdout instead of the remote gitops repo.",
	)

	cmd.Flags().StringVarP(
		&o.TargetBranch,
		flagTargetBranch,
		"t",
		"",
		"The branch of the remote gitops repository to write rendered manifests into.",
	)
	if err := cmd.MarkFlagRequired(flagTargetBranch); err != nil {
		panic(fmt.Errorf("could not mark %s flag as required", flagTargetBranch))
	}

	// Make sure input source is specified and unambiguous.
	cmd.MarkFlagsOneRequired(flagRepo, flagLocalInPath)
	cmd.MarkFlagsMutuallyExclusive(flagRepo, flagLocalInPath)
	// And the ref flag cannot be combined with the local input path..
	cmd.MarkFlagsMutuallyExclusive(flagRef, flagLocalInPath)

	// Make sure output destination is unambiguous.
	cmd.MarkFlagsMutuallyExclusive(flagCommitMessage, flagLocalOutPath, flagStdout)
}

func (o *rootOptions) preRun(cmd *cobra.Command, _ []string) {
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

// run performs manifest rendering.
func (o *rootOptions) run(ctx context.Context, out io.Writer) error {
	logLevel := render.LogLevelError
	if o.debug {
		logLevel = render.LogLevelDebug
	}

	svc := render.NewService(
		&render.ServiceOptions{
			LogLevel: logLevel,
		},
	)

	res, err := svc.RenderManifests(ctx, o.Request)
	if err != nil {
		return err
	}

	if o.outputFormat == "" {
		switch res.ActionTaken {
		case render.ActionTakenNone:
			if o.Stdout {
				return manifestsToStdout(res.Manifests, out)
			}
			fmt.Fprintln(
				out,
				"\nThis request would not change any state. No action was taken.",
			)
		case render.ActionTakenOpenedPR:
			fmt.Fprintf(
				out,
				"\nOpened PR %s\n",
				res.PullRequestURL,
			)
		case render.ActionTakenPushedDirectly:
			fmt.Fprintf(
				out,
				"\nCommitted %s to branch %s\n",
				res.CommitID,
				o.TargetBranch,
			)
		case render.ActionTakenUpdatedPR:
			fmt.Fprintf(
				out,
				"\nUpdated PR %s\n",
				res.PullRequestURL,
			)
		case render.ActionTakenWroteToLocalPath:
			fmt.Fprintf(
				out,
				"\nWrote rendered manifests to %s\n",
				o.LocalOutPath,
			)
		}
	} else {
		if err := output(res, out, o.outputFormat); err != nil {
			return err
		}
	}

	return nil
}

func manifestsToStdout(manifests map[string][]byte, out io.Writer) error {
	apps := make([]string, 0, len(manifests))
	for k := range manifests {
		apps = append(apps, k)
	}
	sort.StringSlice(apps).Sort()
	for _, app := range apps {
		const sep = "--------------------------------------------------"
		fmt.Fprintln(out, sep)
		fmt.Fprintf(out, "App: %s\n", app)
		fmt.Fprintln(out, sep)
		fmt.Fprintln(out, string(manifests[app]))
	}
	return nil
}
