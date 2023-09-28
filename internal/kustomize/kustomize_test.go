package kustomize

import (
	"testing"

	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/require"
)

func TestBuildKustomizeOptions(t *testing.T) {
	testCases := []struct {
		name           string
		enableHelm     bool
		loadRestrictor string
		expected       *argoappv1.KustomizeOptions
	}{
		{
			name:           "Enable helm, no load restrictor",
			enableHelm:     true,
			loadRestrictor: "",
			expected: &argoappv1.KustomizeOptions{
				BuildOptions: "--enable-helm --load-restrictor LoadRestrictionsRootOnly", // nolint:all
			},
		},
		{
			name:           "Disable helm, provide load restrictor",
			enableHelm:     false,
			loadRestrictor: "LoadRestrictionsNone",
			expected: &argoappv1.KustomizeOptions{
				BuildOptions: "--load-restrictor LoadRestrictionsNone",
			},
		},
		{
			name:           "Disable helm, no load restrictor",
			enableHelm:     false,
			loadRestrictor: "",
			expected: &argoappv1.KustomizeOptions{
				BuildOptions: "--load-restrictor LoadRestrictionsRootOnly",
			},
		},
		{
			name:           "Enable helm, provide load restrictor",
			enableHelm:     true,
			loadRestrictor: "LoadRestrictionsNone",
			expected: &argoappv1.KustomizeOptions{
				BuildOptions: "--enable-helm --load-restrictor LoadRestrictionsNone",
			},
		},
	}

	for _, tc := range testCases {
		actual := BuildKustomizeOptions(tc.enableHelm, tc.loadRestrictor)
		require.Equal(t, tc.expected.BuildOptions, actual.BuildOptions)
	}
}
