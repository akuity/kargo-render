package bookkeeper

// ActionTaken indicates what action, if any was taken in response to a
// RenderRequest.
type ActionTaken string

const (
	// ActionTakenNone represents the case where Bookkeeper responded
	// to a RenderRequest by, effectively, doing nothing. This occurs in cases
	// where the fully rendered manifests that would have been written to the
	// target branch do not differ from what is already present at the head of
	// that branch.
	ActionTakenNone ActionTaken = "NONE"
	// ActionTakenOpenedPR represents the case where Bookkeeper responded to a
	// RenderRequest by opening a new pull request against the target branch.
	ActionTakenOpenedPR ActionTaken = "OPENED_PR"
	// ActionTakenPushedDirectly represents the case where Bookkeeper responded
	// to a RenderRequest by pushing a new commit directly to the target branch.
	ActionTakenPushedDirectly ActionTaken = "PUSHED_DIRECTLY"
	// ActionTakenUpdatedPR represents the case where Bookkeeper responded to a
	// RenderRequest by updating an existing PR.
	ActionTakenUpdatedPR ActionTaken = "UPDATED_PR"
)

// RenderRequest is a request for Bookkeeper to render environment-specific
// manifests from input in the  default branch of the repository specified by
// RepoURL.
type RenderRequest struct {
	id string
	// RepoURL is the URL of a remote GitOps repository.
	RepoURL string `json:"repoURL,omitempty"`
	// RepoCreds encapsulates read/write credentials for the remote GitOps
	// repository referenced by the RepoURL field.
	RepoCreds RepoCredentials `json:"repoCreds,omitempty"`
	// Ref specifies either a branch or a precise commit to render manifests from.
	// When this is omitted, the request is assumed to be one to render from the
	// head of the default branch.
	Ref string `json:"ref,omitempty"`
	// TargetBranch is the name of an environment-specific branch in the GitOps
	// repository referenced by the RepoURL field into which plain YAML should be
	// rendered.
	TargetBranch string `json:"targetBranch,omitempty"`
	// Images specifies images to incorporate into environment-specific
	// manifests.
	Images []string `json:"images,omitempty"`
	// CommitMessage offers the opportunity to, optionally, override the first
	// line of the commit message that Bookkeeper would normally generate.
	CommitMessage string `json:"commitMessage,omitempty"`
}

// RepoCredentials represents the credentials for connecting to a private git
// repository.
type RepoCredentials struct {
	// SSHPrivateKey is a private key that can be used for both reading from and
	// writing to some remote repository.
	SSHPrivateKey string `json:"sshPrivateKey,omitempty"`
	// Username identifies a principal, which combined with the value of the
	// Password field, can be used for both reading from and writing to some
	// remote repository.
	Username string `json:"username,omitempty"`
	// Password, when combined with the principal identified by the Username
	// field, can be used for both reading from and writing to some remote
	// repository.
	Password string `json:"password,omitempty"`
}

// RenderResponse encapsulates details of a successful rendering of some
// environment-specific manifests into an environment-specific branch.
type RenderResponse struct {
	ActionTaken ActionTaken `json:"actionTaken,omitempty"`
	// CommitID is the ID (sha) of the commit to the environment-specific branch
	// containing the rendered manifests. This is only set when the OpenPR field
	// of the corresponding RenderRequest was false.
	CommitID string `json:"commitID,omitempty"`
	// PullRequestURL is a URL for a pull request containing the rendered
	// manifests. This is only set when the OpenPR field of the corresponding
	// RenderRequest was true.
	PullRequestURL string `json:"pullRequestURL,omitempty"`
}
