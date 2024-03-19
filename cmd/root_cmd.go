package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	render "github.com/akuity/kargo-render"
)

type rootOptions struct {
	render.Request
	commitMessage string
	debug         bool
	outputFormat  string
}

func newRootCommand() *cobra.Command {
	cmdOpts := &rootOptions{}

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
			if err := cmdOpts.validate(); err != nil {
				return err
			}
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
		"allow the rendered manifests to be empty; if false this is disallowed as "+
			"a safeguard against scenarios where a bug of any kind might otherwise "+
			"cause Kargo Render to wipe out the contents of the target branch in "+
			"error",
	)

	cmd.Flags().StringVarP(
		&o.commitMessage,
		flagCommitMessage,
		"m",
		"",
		"specify a custom message to be used for the commit to the target branch",
	)

	cmd.Flags().BoolVarP(
		&o.debug,
		flagDebug,
		"d",
		false,
		"display debug output",
	)

	cmd.Flags().StringArrayVarP(
		&o.Images,
		flagImage,
		"i",
		nil,
		"specify a new image to apply to the final result (this flag may be "+
			"used more than once)",
	)

	cmd.Flags().StringVarP(
		&o.outputFormat,
		flagOutput,
		"o",
		"",
		"specify a format for command output (json or yaml)",
	)

	cmd.Flags().StringVarP(
		&o.Ref,
		flagRef,
		"R",
		"",
		"specify a branch or a precise commit to render from; if this is not "+
			"provided, Kargo Render renders from HEAD",
	)

	cmd.Flags().StringVarP(
		&o.RepoURL,
		flagRepo,
		"r",
		"",
		"the URL of a remote gitops repo (required)",
	)
	if err := cmd.MarkFlagRequired(flagRepo); err != nil {
		panic(fmt.Errorf("could not mark %s flag as required", flagRepo))
	}

	cmd.Flags().StringVarP(
		&o.RepoCreds.Password,
		flagRepoPassword,
		"p",
		"",
		"password or token for reading from and writing to the remote gitops "+
			"repo (required; can also be set using the KARGO_RENDER_REPO_PASSWORD "+
			"environment variable)",
	)
	if err := cmd.MarkFlagRequired(flagRepoPassword); err != nil {
		panic(fmt.Errorf("could not mark %s flag as required", flagRepoPassword))
	}

	cmd.Flags().StringVarP(
		&o.RepoCreds.Username,
		flagRepoUsername,
		"u",
		"",
		"username for reading from and writing to the remote gitops repo "+
			"(required; can also be set using the KARGO_RENDER_REPO_USERNAME "+
			"environment variable)",
	)
	if err := cmd.MarkFlagRequired(flagRepoUsername); err != nil {
		panic(fmt.Errorf("could not mark %s flag as required", flagRepoUsername))
	}

	cmd.Flags().StringVarP(
		&o.TargetBranch,
		flagTargetBranch,
		"t",
		"",
		"the branch to render manifests into (required)",
	)
	if err := cmd.MarkFlagRequired(flagTargetBranch); err != nil {
		panic(fmt.Errorf("could not mark %s flag as required", flagTargetBranch))
	}
}

// validate performs validation of the options. If the options are invalid, an
// error is returned.
func (o *rootOptions) validate() error {
	var errs []error
	// While these flags are marked as required, a user could still provide an
	// empty string for any of them. This is a check to ensure that required flags
	// are not empty.
	if o.RepoURL == "" {
		errs = append(errs, fmt.Errorf("the --%s flag is required", flagRepo))
	}
	if o.RepoCreds.Password == "" {
		errs = append(errs, fmt.Errorf("the --%s flag is required", flagRepoPassword))
	}
	if o.RepoCreds.Username == "" {
		errs = append(errs, fmt.Errorf("the --%s flag is required", flagRepoUsername))
	}
	if o.TargetBranch == "" {
		errs = append(errs, fmt.Errorf("the --%s flag is required", flagTargetBranch))
	}
	return errors.Join(errs...)
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

		}
	} else {
		if err := output(res, out, o.outputFormat); err != nil {
			return err
		}
	}

	return nil
}
