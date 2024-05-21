package github

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseGithubURL(t *testing.T) {
	url := "https://github.com/foo/bar"
	var enterprise bool
	var baseURL string
	var exists string
	var err error

	enterprise, baseURL, exists, _, err = parseGitHubURL(url)
	require.NoError(t, err)
	require.Equal(t, false, enterprise)
	require.Equal(t, "https://github.com", baseURL)
	require.Equal(t, "foo", exists)

	url = "https://mygithub.co.uk/foo-BAR/bar"
	// url = "https://otherdomain.example/baz/bar"
	enterprise, baseURL, exists, _, err = parseGitHubURL(url)
	require.NoError(t, err)
	require.Equal(t, true, enterprise)
	require.Equal(t, "https://mygithub.co.uk", baseURL)
	require.Equal(t, "foo-BAR", exists)
	// file = "bogus.go"
	// exists, err = Exists(file)
	// require.NoError(t, err)
	// require.False(t, exists)
}
