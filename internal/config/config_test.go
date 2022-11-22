package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadRepoConfig(t *testing.T) {
	testCases := []struct {
		name       string
		setup      func() string
		assertions func(error)
	}{
		{
			name: "invalid JSON",
			setup: func() string {
				dir, err := os.MkdirTemp("", "")
				require.NoError(t, err)
				err = os.WriteFile(
					filepath.Join(dir, "Bookfile.json"),
					[]byte("bogus"),
					0600,
				)
				require.NoError(t, err)
				return dir
			},
			assertions: func(err error) {
				require.Error(t, err)
				require.Contains(
					t,
					err.Error(),
					"error normalizing and validating Bookkeeper configuration",
				)
			},
		},
		{
			name: "invalid YAML",
			setup: func() string {
				dir, err := os.MkdirTemp("", "")
				require.NoError(t, err)
				err = os.WriteFile(
					filepath.Join(dir, "Bookfile.yaml"),
					[]byte("bogus"),
					0600,
				)
				require.NoError(t, err)
				return dir
			},
			assertions: func(err error) {
				require.Error(t, err)
				require.Contains(
					t,
					err.Error(),
					"error normalizing and validating Bookkeeper configuration",
				)
			},
		},
		{
			name: "valid JSON",
			setup: func() string {
				dir, err := os.MkdirTemp("", "")
				require.NoError(t, err)
				err = os.WriteFile(
					filepath.Join(dir, "Bookfile.json"),
					[]byte(`{"configVersion": "v1alpha1"}`),
					0600,
				)
				require.NoError(t, err)
				return dir
			},
			assertions: func(err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "valid YAML",
			setup: func() string {
				dir, err := os.MkdirTemp("", "")
				require.NoError(t, err)
				err = os.WriteFile(
					filepath.Join(dir, "Bookfile.yaml"),
					[]byte("configVersion: v1alpha1"),
					0600,
				)
				require.NoError(t, err)
				return dir
			},
			assertions: func(err error) {
				require.NoError(t, err)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := LoadRepoConfig(testCase.setup())
			testCase.assertions(err)
		})
	}
}

func TestGetBranchConfig(t *testing.T) {
	const testBranchName = "foo"
	testCases := []struct {
		name       string
		repoConfig RepoConfig
		assertions func(BranchConfig)
	}{
		{
			name: "branch config explicitly specified",
			repoConfig: &repoConfig{
				BranchConfigs: []BranchConfig{
					{
						Name: testBranchName,
						ConfigManagement: ConfigManagementConfig{
							Ytt: &YttConfig{},
						},
					},
				},
			},
			assertions: func(cfg BranchConfig) {
				require.Equal(t, testBranchName, cfg.Name)
				require.Nil(t, cfg.ConfigManagement.Helm)
				require.Nil(t, cfg.ConfigManagement.Kustomize)
				require.NotNil(t, cfg.ConfigManagement.Ytt)
			},
		},
		{
			name: "default branch config explicitly specified",
			repoConfig: &repoConfig{
				DefaultBranchConfig: &BranchConfig{
					ConfigManagement: ConfigManagementConfig{
						Ytt: &YttConfig{},
					},
				},
			},
			assertions: func(cfg BranchConfig) {
				require.Equal(t, testBranchName, cfg.Name)
				require.Nil(t, cfg.ConfigManagement.Helm)
				require.Nil(t, cfg.ConfigManagement.Kustomize)
				require.NotNil(t, cfg.ConfigManagement.Ytt)
			},
		},
		{
			name:       "nothing explicitly specified",
			repoConfig: &repoConfig{},
			assertions: func(cfg BranchConfig) {
				require.Equal(t, testBranchName, cfg.Name)
				require.Nil(t, cfg.ConfigManagement.Helm)
				require.NotNil(t, cfg.ConfigManagement.Kustomize)
				require.Nil(t, cfg.ConfigManagement.Ytt)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.assertions(
				testCase.repoConfig.GetBranchConfig(testBranchName),
			)
		})
	}
}

func TestNormalizeAndValidate(t *testing.T) {
	testCases := []struct {
		name       string
		config     []byte
		assertions func(error)
	}{
		{
			name:   "invalid JSON",
			config: []byte("{}"),
			assertions: func(err error) {
				require.Error(t, err)
			},
		},
		{
			name:   "invalid YAML",
			config: []byte(""),
			assertions: func(err error) {
				require.Error(t, err)
			},
		},
		{
			name:   "valid JSON",
			config: []byte(`{"configVersion": "v1alpha1"}`),
			assertions: func(err error) {
				require.NoError(t, err)
			},
		},
		{
			name:   "valid YAML",
			config: []byte("configVersion: v1alpha1"),
			assertions: func(err error) {
				require.NoError(t, err)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			configBytes, err := normalizeAndValidate(testCase.config)
			testCase.assertions(err)
			// For any validation that doesn't fail, the bytes returned should be
			// JSON we can unmarshal...
			if err == nil {
				cfg := repoConfig{}
				err = json.Unmarshal(configBytes, &cfg)
				require.NoError(t, err)
			}
		})
	}
}
