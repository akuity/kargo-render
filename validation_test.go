package render

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateAndCanonicalizeRequest(t *testing.T) {
	testCases := []struct {
		name       string
		req        Request
		assertions func(*testing.T, Request, error)
	}{
		{
			name: "no input source specified",
			req:  Request{},
			assertions: func(t *testing.T, _ Request, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "no input source specified")
			},
		},
		{
			name: "input source is ambiguous",
			req: Request{
				RepoURL:     "fake-url",
				LocalInPath: "/some/path",
			},
			assertions: func(t *testing.T, _ Request, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "input source is ambiguous")
			},
		},
		{
			name: "local input path and git ref incorrectly used together",
			req: Request{
				LocalInPath: "/some/path",
				Ref:         "1abcdef2",
			},
			assertions: func(t *testing.T, _ Request, err error) {
				require.Error(t, err)
				require.Contains(
					t,
					err.Error(),
					"LocalInPath and Ref are mutually exclusive",
				)
			},
		},
		{
			name: "output destination is ambiguous",
			req: Request{
				LocalOutPath: "/some/path",
				Stdout:       true,
			},
			assertions: func(t *testing.T, _ Request, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "output destination is ambiguous")
			},
		},
		{
			name: "invalid RepoURL",
			req: Request{
				RepoURL: "fake-url",
			},
			assertions: func(t *testing.T, _ Request, err error) {
				require.Error(t, err)
				require.Contains(
					t,
					err.Error(),
					"does not appear to be a valid git repository URL",
				)
			},
		},
		{
			name: "missing TargetBranch",
			req: Request{
				RepoURL: "https://github.com/akuity/foobar",
				RepoCreds: RepoCredentials{
					Password: "foobar",
				},
				Ref: "1abcdef2",
			},
			assertions: func(t *testing.T, _ Request, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "TargetBranch is a required field")
			},
		},
		{
			name: "invalid TargetBranch",
			req: Request{
				RepoURL: "https://github.com/akuity/foobar",
				RepoCreds: RepoCredentials{
					Password: "foobar",
				},
				Ref:          "1abcdef2",
				TargetBranch: "env/dev*", // * is an invalid character
			},
			assertions: func(t *testing.T, _ Request, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "is an invalid branch name")
			},
		},
		{
			name: "empty string image",
			req: Request{
				RepoURL: "https://github.com/akuity/foobar",
				RepoCreds: RepoCredentials{
					Password: "foobar",
				},
				Ref:          "1abcdef2",
				TargetBranch: "env/dev",
				Images:       []string{""}, // no good
			},
			assertions: func(t *testing.T, _ Request, err error) {
				require.Error(t, err)
				require.Contains(
					t,
					err.Error(),
					"Images must not contain any empty strings",
				)
			},
		},
		{
			name: "LocalInPath does not exist",
			req: Request{
				LocalInPath: "/some/path/that/does/not/exist",
			},
			assertions: func(t *testing.T, _ Request, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "does not exist")
			},
		},
		{
			name: "LocalOutPath exists",
			req: Request{
				LocalOutPath: t.TempDir(),
			},
			assertions: func(t *testing.T, _ Request, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "already exists; refusing to overwrite")
			},
		},
		{
			name: "validation succeeds",
			req: Request{
				RepoURL: "  https://github.com/akuity/foobar  ",
				RepoCreds: RepoCredentials{
					Password: "  foobar  ",
				},
				Ref:          "  1abcdef2 ",
				TargetBranch: "  refs/heads/env/dev  ",
				Images:       []string{" akuity/some-image "}, // no good
			},
			assertions: func(t *testing.T, req Request, err error) {
				require.NoError(t, err)
				require.Equal(t, "https://github.com/akuity/foobar", req.RepoURL)
				require.Equal(t, "foobar", req.RepoCreds.Password)
				require.Equal(t, "1abcdef2", req.Ref)
				require.Equal(t, "env/dev", req.TargetBranch)
				require.Equal(t, []string{"akuity/some-image"}, req.Images)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.req.canonicalizeAndValidate()
			testCase.assertions(t, testCase.req, err)
		})
	}
}
