package git

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/sosedoff/gitkit"
	"github.com/stretchr/testify/require"

	libOS "github.com/akuity/kargo-render/internal/os"
)

func TestRepo(t *testing.T) {
	testRepoCreds := RepoCredentials{
		Username: "fake-username",
		Password: "fake-password",
	}

	// This will be something to opt into because on some OSes, this will lead
	// to keychain-related prompts.
	useAuth, err := libOS.GetBoolFromEnvVar("TEST_GIT_CLIENT_WITH_AUTH", false)
	require.NoError(t, err)
	service := gitkit.New(
		gitkit.Config{
			Dir:        t.TempDir(),
			AutoCreate: true,
			Auth:       useAuth,
		},
	)
	require.NoError(t, service.Setup())
	service.AuthFunc =
		func(cred gitkit.Credential, _ *gitkit.Request) (bool, error) {
			return cred.Username == testRepoCreds.Username &&
				cred.Password == testRepoCreds.Password, nil
		}
	server := httptest.NewServer(service)
	defer server.Close()

	testRepoURL := fmt.Sprintf("%s/test.git", server.URL)

	rep, err := Clone(testRepoURL, testRepoCreds)
	require.NoError(t, err)
	require.NotNil(t, rep)
	r, ok := rep.(*repo)
	require.True(t, ok)

	t.Run("can clone", func(t *testing.T) {
		var repoURL *url.URL
		repoURL, err = url.Parse(r.url)
		require.NoError(t, err)
		repoURL.User = nil
		require.Equal(t, testRepoURL, repoURL.String())
		require.NotEmpty(t, r.homeDir)
		var fi os.FileInfo
		fi, err = os.Stat(r.homeDir)
		require.NoError(t, err)
		require.True(t, fi.IsDir())
		require.NotEmpty(t, r.dir)
		fi, err = os.Stat(r.dir)
		require.NoError(t, err)
		require.True(t, fi.IsDir())
		require.Equal(t, "HEAD", r.currentBranch)
	})

	t.Run("can get the repo url", func(t *testing.T) {
		require.Equal(t, r.url, r.URL())
	})

	t.Run("can get the home dir", func(t *testing.T) {
		require.Equal(t, r.homeDir, r.HomeDir())
	})

	t.Run("can get the working dir", func(t *testing.T) {
		require.Equal(t, r.dir, r.WorkingDir())
	})

	t.Run("can list remotes", func(t *testing.T) {
		var remotes []string
		remotes, err = r.Remotes()
		require.NoError(t, err)
		require.Len(t, remotes, 1)
		require.Equal(t, RemoteOrigin, remotes[0])
	})

	t.Run("can get url of a remote", func(t *testing.T) {
		var url string
		url, err = r.RemoteURL(RemoteOrigin)
		require.NoError(t, err)
		require.Equal(t, r.url, url)
	})

	t.Run("can check for diffs -- negative result", func(t *testing.T) {
		var hasDiffs bool
		hasDiffs, err = r.HasDiffs()
		require.NoError(t, err)
		require.False(t, hasDiffs)
	})

	err = os.WriteFile(fmt.Sprintf("%s/%s", r.WorkingDir(), "test.txt"), []byte("foo"), 0600)
	require.NoError(t, err)

	t.Run("can check for diffs -- positive result", func(t *testing.T) {
		var hasDiffs bool
		hasDiffs, err = r.HasDiffs()
		require.NoError(t, err)
		require.True(t, hasDiffs)
	})

	t.Run("can get diff paths", func(t *testing.T) {
		var paths []string
		paths, err = r.GetDiffPaths()
		require.NoError(t, err)
		require.Len(t, paths, 1)
	})

	testCommitMessage := fmt.Sprintf("test commit %s", uuid.NewString())
	err = r.AddAllAndCommit(testCommitMessage)
	require.NoError(t, err)

	t.Run("can commit", func(t *testing.T) {
		require.NoError(t, err)
	})

	lastCommitID, err := r.LastCommitID()
	require.NoError(t, err)

	t.Run("can get last commit id", func(t *testing.T) {
		require.NoError(t, err)
		require.NotEmpty(t, lastCommitID)
	})

	t.Run("can get commit message by id", func(t *testing.T) {
		var msg string
		msg, err = r.CommitMessage(lastCommitID)
		require.NoError(t, err)
		require.Equal(t, testCommitMessage, msg)
	})

	t.Run("can check if remote branch exists -- negative result", func(t *testing.T) {
		var exists bool
		exists, err = r.RemoteBranchExists("main") // The remote repo is empty!
		require.NoError(t, err)
		require.False(t, exists)
	})

	err = r.Push()
	require.NoError(t, err)

	t.Run("can push", func(t *testing.T) {
		require.NoError(t, err)
	})

	t.Run("can check if remote branch exists -- positive result", func(t *testing.T) {
		var exists bool
		// "master" is still the default branch name for a new repository unless
		// you configure it otherwise.
		exists, err = r.RemoteBranchExists("master")
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("can fetch", func(t *testing.T) {
		err = r.Fetch()
		require.NoError(t, err)
	})

	t.Run("can pull", func(t *testing.T) {
		err = r.Pull("master")
		require.NoError(t, err)
	})

	testBranch := fmt.Sprintf("test-branch-%s", uuid.NewString())
	err = r.CreateChildBranch(testBranch)
	require.NoError(t, err)

	t.Run("can create a child branch", func(t *testing.T) {
		require.NoError(t, err)
	})

	t.Run("can check if local branch exists -- negative result", func(t *testing.T) {
		var exists bool
		exists, err = r.LocalBranchExists("branch-that-does-not-exist")
		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("can check if local branch exists -- positive result", func(t *testing.T) {
		var exists bool
		exists, err = r.LocalBranchExists(testBranch)
		require.NoError(t, err)
		require.True(t, exists)
	})

	err = os.WriteFile(fmt.Sprintf("%s/%s", r.WorkingDir(), "test.txt"), []byte("bar"), 0600)
	require.NoError(t, err)

	t.Run("can hard reset", func(t *testing.T) {
		var hasDiffs bool
		hasDiffs, err = r.HasDiffs()
		require.NoError(t, err)
		require.True(t, hasDiffs)
		err = r.ResetHard()
		require.NoError(t, err)
		hasDiffs, err = r.HasDiffs()
		require.NoError(t, err)
		require.False(t, hasDiffs)
	})

	t.Run("can create an orphaned branch", func(t *testing.T) {
		testBranch := fmt.Sprintf("test-branch-%s", uuid.NewString())
		err = r.CreateOrphanedBranch(testBranch)
		require.NoError(t, err)
	})

	t.Run("can copy an existing repo", func(t *testing.T) {
		newRepo, err := CopyRepo(r.WorkingDir(), testRepoCreds)
		require.NoError(t, err)
		defer newRepo.Close()
		require.NotNil(t, newRepo)
		require.Equal(t, r.URL(), r.URL())
		require.NotEqual(t, r.HomeDir(), newRepo.HomeDir())
		fi, err := os.Stat(newRepo.HomeDir())
		require.NoError(t, err)
		require.True(t, fi.IsDir())
		require.NotEqual(t, r.WorkingDir(), newRepo.WorkingDir())
		fi, err = os.Stat(newRepo.WorkingDir())
		require.NoError(t, err)
		require.True(t, fi.IsDir())
	})

	t.Run("can close repo", func(t *testing.T) {
		require.NoError(t, r.Close())
		_, err := os.Stat(r.HomeDir())
		require.Error(t, err)
		require.True(t, os.IsNotExist(err))
	})

}
