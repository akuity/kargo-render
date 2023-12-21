package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	render "github.com/akuity/kargo-render"
)

func newRenderCommand() (*cobra.Command, error) {
	const desc = "Render environment-specific manifests into an " +
		"environment-specific branch of the remote gitops repo"
	cmd := &cobra.Command{
		Use:   "render",
		Short: desc,
		Long:  desc,
		RunE:  runRenderCmd,
	}
	cmd.Flags().AddFlagSet(flagSetOutput)
	cmd.Flags().StringP(
		flagRef,
		"R",
		"",
		"specify a branch or a precise commit to render from; if this is not "+
			"provided, Kargo Render renders from the head of the default branch",
	)
	cmd.Flags().String(
		flagRefPath,
		"",
		"a path in the ref branch/commit to render from; if unspecified, "+
			"uses target-branch as the path",
	)
	cmd.Flags().StringP(
		flagCommitMessage,
		"m",
		"",
		"specify a custom message to be used for the commit to the target branch",
	)
	cmd.Flags().BoolP(
		flagDebug,
		"d",
		false,
		"display debug output",
	)
	cmd.Flags().StringArrayP(
		flagImage,
		"i",
		nil,
		"specify a new image to apply to the final result (this flag may be "+
			"used more than once)",
	)
	cmd.Flags().StringP(
		flagRepo,
		"r",
		"",
		"the URL of a remote gitops repo (required)",
	)
	if err := cmd.MarkFlagRequired(flagRepo); err != nil {
		return nil, err
	}
	cmd.Flags().StringP(
		flagRepoPassword,
		"p",
		"",
		"password or token for reading from and writing to the remote gitops "+
			"repo (required; can also be set using the KARGO_RENDER_REPO_PASSWORD "+
			"environment variable)",
	)
	if err := cmd.MarkFlagRequired(flagRepoPassword); err != nil {
		return nil, err
	}
	cmd.Flags().StringP(
		flagRepoUsername,
		"u",
		"",
		"username for reading from and writing to the remote gitops repo "+
			"(required; can also be set using the KARGO_RENDER_REPO_USERNAME "+
			"environment variable)",
	)
	if err := cmd.MarkFlagRequired(flagRepoUsername); err != nil {
		return nil, err
	}
	cmd.Flags().StringP(
		flagTargetBranch,
		"t",
		"",
		"the environment-specific branch to render manifests into (required)",
	)
	cmd.Flags().Bool(
		flagAllowEmpty,
		false,
		"allow the rendered manifests to be empty; if false this is disallowed as "+
			"a safeguard against scenarios where a bug of any kind might otherwise "+
			"cause Kargo Render to wipe out the contents of the target branch in "+
			"error",
	)
	if err := cmd.MarkFlagRequired(flagTargetBranch); err != nil {
		return nil, err
	}
	return cmd, nil
}

func runRenderCmd(cmd *cobra.Command, _ []string) error {
	req := render.Request{}
	var err error
	req.Images, err = cmd.Flags().GetStringArray(flagImage)
	if err != nil {
		return err
	}
	req.RepoURL, err = cmd.Flags().GetString(flagRepo)
	if err != nil {
		return err
	}
	req.RepoCreds.Username, err = cmd.Flags().GetString(flagRepoUsername)
	if err != nil {
		return err
	}
	req.RepoCreds.Password, err = cmd.Flags().GetString(flagRepoPassword)
	if err != nil {
		return err
	}
	req.Ref, err = cmd.Flags().GetString(flagRef)
	if err != nil {
		return err
	}
	req.RefPath, err = cmd.Flags().GetString(flagRefPath)
	if err != nil {
		return err
	}
	req.TargetBranch, err = cmd.Flags().GetString(flagTargetBranch)
	if err != nil {
		return err
	}
	req.CommitMessage, err = cmd.Flags().GetString(flagCommitMessage)
	if err != nil {
		return err
	}
	req.AllowEmpty, err = cmd.Flags().GetBool(flagAllowEmpty)
	if err != nil {
		return err
	}

	logLevel := render.LogLevelError
	var debug bool
	if debug, err = cmd.Flags().GetBool(flagDebug); err != nil {
		return err
	}
	if debug {
		logLevel = render.LogLevelDebug
	}
	svc := render.NewService(
		&render.ServiceOptions{
			LogLevel: logLevel,
		},
	)

	res, err := svc.RenderManifests(cmd.Context(), req)
	if err != nil {
		return err
	}

	outputFormat, err := cmd.Flags().GetString(flagOutput)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if outputFormat == "" {
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
				req.TargetBranch,
			)
		case render.ActionTakenUpdatedPR:

		}
	} else {
		if err := output(res, out, outputFormat); err != nil {
			return err
		}
	}

	return nil
}
