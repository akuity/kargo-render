package cli

import (
	"flag"

	"github.com/spf13/pflag"
)

const (
	flagAllowEmpty    = "allow-empty"
	flagCommitMessage = "commit-message"
	flagDebug         = "debug"
	flagImage         = "image"
	flagOutput        = "output"
	flagOutputJSON    = "json"
	flagOutputYAML    = "yaml"
	flagRef           = "ref"
	flagRepo          = "repo"
	flagRepoPassword  = "repo-password"
	flagRepoUsername  = "repo-username"
	flagTargetBranch  = "target-branch"
)

var flagSetOutput *pflag.FlagSet

func init() {
	flagSetOutput = pflag.NewFlagSet(
		"output",
		pflag.ErrorHandling(flag.ExitOnError),
	)
	flagSetOutput.StringP(
		flagOutput,
		"o",
		"",
		"specify a format for command output (json or yaml)",
	)
}
