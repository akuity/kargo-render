package bookkeeper

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/akuityio/bookkeeper/internal/file"
	"github.com/stretchr/testify/require"
)

func TestValidateAndCanonicalizeRequest(t *testing.T) {
	testCases := []struct {
		name       string
		req        RenderRequest
		assertions func(RenderRequest, error)
	}{
		{
			name: "missing RepoURL",
			req:  RenderRequest{},
			assertions: func(req RenderRequest, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "RepoURL is a required field")
			},
		},
		{
			name: "invalid RepoURL",
			req: RenderRequest{
				RepoURL: "foobar",
			},
			assertions: func(req RenderRequest, err error) {
				require.Error(t, err)
				require.Contains(
					t,
					err.Error(),
					"does not appear to be a valid git repository URL",
				)
			},
		},
		{
			name: "missing Password",
			req: RenderRequest{
				RepoURL: "https://github.com/akuityio/foobar",
			},
			assertions: func(req RenderRequest, err error) {
				require.Error(t, err)
				require.Contains(
					t,
					err.Error(),
					"RepoCreds.Password is a required field",
				)
			},
		},
		{
			name: "Commit too short",
			req: RenderRequest{
				RepoURL: "https://github.com/akuityio/foobar",
				RepoCreds: RepoCredentials{
					Password: "foobar",
				},
				Commit: "abcd",
			},
			assertions: func(req RenderRequest, err error) {
				require.Error(t, err)
				require.Contains(
					t,
					err.Error(),
					"does not appear to be a valid commit ID",
				)
			},
		},
		{
			name: "Commit too long",
			req: RenderRequest{
				RepoURL: "https://github.com/akuityio/foobar",
				RepoCreds: RepoCredentials{
					Password: "foobar",
				},
				Commit: "01234567890123456789012345678901234567890", // 41 characters
			},
			assertions: func(req RenderRequest, err error) {
				require.Error(t, err)
				require.Contains(
					t,
					err.Error(),
					"does not appear to be a valid commit ID",
				)
			},
		},
		{
			name: "Commit contains invalid characters",
			req: RenderRequest{
				RepoURL: "https://github.com/akuityio/foobar",
				RepoCreds: RepoCredentials{
					Password: "foobar",
				},
				Commit: "lorem ipsum", // non hex characters
			},
			assertions: func(req RenderRequest, err error) {
				require.Error(t, err)
				require.Contains(
					t,
					err.Error(),
					"does not appear to be a valid commit ID",
				)
			},
		},
		{
			name: "missing TargetBranch",
			req: RenderRequest{
				RepoURL: "https://github.com/akuityio/foobar",
				RepoCreds: RepoCredentials{
					Password: "foobar",
				},
				Commit: "1abcdef2",
			},
			assertions: func(req RenderRequest, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "TargetBranch is a required field")
			},
		},
		{
			name: "invalid TargetBranch",
			req: RenderRequest{
				RepoURL: "https://github.com/akuityio/foobar",
				RepoCreds: RepoCredentials{
					Password: "foobar",
				},
				Commit:       "1abcdef2",
				TargetBranch: "env/dev*", // * is an invalid character
			},
			assertions: func(req RenderRequest, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "is an invalid branch name")
			},
		},
		{
			name: "empty string image",
			req: RenderRequest{
				RepoURL: "https://github.com/akuityio/foobar",
				RepoCreds: RepoCredentials{
					Password: "foobar",
				},
				Commit:       "1abcdef2",
				TargetBranch: "env/dev",
				Images:       []string{""}, // no good
			},
			assertions: func(req RenderRequest, err error) {
				require.Error(t, err)
				require.Contains(
					t,
					err.Error(),
					"Images must not contain any empty strings",
				)
			},
		},
		{
			name: "validation succeeds",
			req: RenderRequest{
				RepoURL: "  https://github.com/akuityio/foobar  ",
				RepoCreds: RepoCredentials{
					Password: "  foobar  ",
				},
				Commit:       "  1abcdef2 ",
				TargetBranch: "  refs/heads/env/dev  ",
				Images:       []string{" akuityio/some-image "}, // no good
			},
			assertions: func(req RenderRequest, err error) {
				require.NoError(t, err)
				require.Equal(t, "https://github.com/akuityio/foobar", req.RepoURL)
				require.Equal(t, "foobar", req.RepoCreds.Password)
				require.Equal(t, "1abcdef2", req.Commit)
				require.Equal(t, "env/dev", req.TargetBranch)
				require.Equal(t, []string{"akuityio/some-image"}, req.Images)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.assertions(validateAndCanonicalizeRequest(testCase.req))
		})
	}
}

func TestWriteFiles(t *testing.T) {
	testYAMLChunk1 := []byte(`kind: Deployment
metadata:
  name: foobar
`)
	testYAMLChunk2 := []byte(`kind: Service
metadata:
  name: foobar
`)
	testYAMLBytes := bytes.Join(
		[][]byte{
			testYAMLChunk1,
			testYAMLChunk2,
		},
		[]byte("---\n"),
	)
	testDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer os.RemoveAll(testDir)
	err = writeFiles(testDir, testYAMLBytes)
	require.NoError(t, err)
	filename := filepath.Join(testDir, "foobar-deployment.yaml")
	exists, err := file.Exists(filename)
	require.NoError(t, err)
	require.True(t, exists)
	fileBytes, err := os.ReadFile(filename)
	require.NoError(t, err)
	require.Equal(t, testYAMLChunk1, fileBytes)
	filename = filepath.Join(testDir, "foobar-service.yaml")
	exists, err = file.Exists(filename)
	require.NoError(t, err)
	require.True(t, exists)
	fileBytes, err = os.ReadFile(filename)
	require.NoError(t, err)
	require.Equal(t, testYAMLChunk2, fileBytes)
}

func TestRmYAML(t *testing.T) {
	testDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer os.RemoveAll(testDir)
	file1 := filepath.Join(testDir, "foo.yaml")
	file2 := filepath.Join(testDir, "bar.yaml")
	_, err = os.Create(file1)
	require.NoError(t, err)
	_, err = os.Create(file2)
	require.NoError(t, err)
	err = rmYAML(testDir)
	require.NoError(t, err)
	exists, err := file.Exists(file1)
	require.NoError(t, err)
	require.False(t, exists)
	exists, err = file.Exists(file2)
	require.NoError(t, err)
	require.False(t, exists)
}
