package argocd

import (
	"testing"

	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/require"
)

func TestExpand(t *testing.T) {
	cfg := ConfigManagementConfig{
		Path: "charts/foo",
		Helm: &ApplicationSourceHelm{
			Namespace: "foo",
			ApplicationSourceHelm: argoappv1.ApplicationSourceHelm{
				ReleaseName: "foo",
				ValueFiles:  []string{"env/${1}/foo/values.yaml"},
				Parameters: []argoappv1.HelmParameter{{
					Name:  "env",
					Value: "${1}",
				}},
			},
		},
	}
	expandedCfg, err := cfg.Expand([]string{"foo", "bar"})
	require.NoError(t, err)

	require.Equal(t, "env/bar/foo/values.yaml", expandedCfg.Helm.ValueFiles[0])
	require.Equal(t, "bar", expandedCfg.Helm.Parameters[0].Value)
}
