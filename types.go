package render

// ActionTaken indicates what action, if any was taken in response to a
// RenderRequest.
type ActionTaken string

const (
	// ActionTakenNone represents the case where Kargo Render responded
	// to a RenderRequest by, effectively, doing nothing. This occurs in cases
	// where the fully rendered manifests that would have been written to the
	// target branch do not differ from what is already present at the head of
	// that branch.
	ActionTakenNone ActionTaken = "NONE"
	// ActionTakenOpenedPR represents the case where Kargo Render responded to a
	// RenderRequest by opening a new pull request against the target branch.
	ActionTakenOpenedPR ActionTaken = "OPENED_PR"
	// ActionTakenPushedDirectly represents the case where Kargo Render responded
	// to a RenderRequest by pushing a new commit directly to the target branch.
	ActionTakenPushedDirectly ActionTaken = "PUSHED_DIRECTLY"
	// ActionTakenUpdatedPR represents the case where Kargo Render responded to a
	// RenderRequest by updating an existing PR.
	ActionTakenUpdatedPR ActionTaken = "UPDATED_PR"
	// ActionTakenWroteToLocalPath represents the case where Kargo Render
	// responded to a RenderRequest by writing the rendered manifests to a local
	// path.
	ActionTakenWroteToLocalPath ActionTaken = "WROTE_TO_LOCAL_PATH"
)

// Request is a request for Kargo Render to render environment-specific
// manifests from input in the  default branch of the repository specified by
// RepoURL.
type Request struct {
	id string
	// RepoURL is the URL of a remote GitOps repository. This field is mutually
	// exclusive with the LocalInPath field.
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
	// line of the commit message that Kargo Render would normally generate.
	CommitMessage string `json:"commitMessage,omitempty"`
	// AllowEmpty indicates whether or not Kargo Render should allow the rendered
	// manifests to be empty. If this is false (the default), Kargo Render will
	// return an error if the rendered manifests are empty. This is a safeguard
	// against scenarios where a bug of any kind might otherwise cause Kargo
	// Render to wipe out the contents of the target branch in error.
	AllowEmpty bool `json:"allowEmpty,omitempty"`
	// LocalInPath specifies a path to the repository's working tree with the
	// desired source commit already checked out. The contents at this path will
	// not be modified. This field is mutually exclusive with the Ref field.
	LocalInPath string `json:"localInPath,omitempty"`
	// LocalOutPath specifies a path where the rendered manifests should be
	// written. The specified path must NOT exist already. When specified, the
	// rendered manifests will not be written to the target branch of the
	// repository specified by the RepoURL field. This field is mutually exclusive
	// with the Stdout field.
	LocalOutPath string `json:"localOutPath,omitempty"`
	// Stdout specifies whether rendered manifests should be written to stdout
	// instead of to the target branch of the repository specified by the RepoURL
	// field. This field is mutually exclusive with the LocalOutPath field.
	Stdout bool `json:"stdout,omitempty"`
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

// Response encapsulates details of a successful rendering of some
// environment-specific manifests into an environment-specific branch.
type Response struct {
	ActionTaken ActionTaken `json:"actionTaken,omitempty"`
	// CommitID is the ID (sha) of the commit to the environment-specific branch
	// containing the rendered manifests. This is only set when the OpenPR field
	// of the corresponding RenderRequest was false.
	CommitID string `json:"commitID,omitempty"`
	// PullRequestURL is a URL for a pull request containing the rendered
	// manifests. This is only set when the OpenPR field of the corresponding
	// RenderRequest was true.
	PullRequestURL string `json:"pullRequestURL,omitempty"`
	// LocalPath is the path to the directory where the rendered manifests
	// were written. This is only set when the LocalOutPath field of the
	// corresponding RenderRequest was non-empty.
	LocalPath string `json:"localPath,omitempty"`
	// Manifests is the rendered environment-specific manifests. This is only set
	// when the Stdout field of the corresponding RenderRequest was true.
	Manifests map[string][]byte `json:"manifests,omitempty"`
}
