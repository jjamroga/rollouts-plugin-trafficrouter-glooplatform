package examples

import (
	_ "embed"
	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform/pkg/util"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	v2 "github.com/solo-io/solo-apis/client-go/networking.gloo.solo.io/v2"
)

var (
	//go:embed 0-rollout-initial-state-green/rollout.yaml
	simpleRollout string

	//go:embed 0-rollout-initial-state-green/route-table.yaml
	simpleRouteTable string
)

func GetSimpleCanaryRollout() *util.CustomResourceWrapper[*v1alpha1.Rollout] {
	return util.FromYaml[*v1alpha1.Rollout](simpleRollout)
}

func GetSimpleRouteTable() *util.CustomResourceWrapper[*v2.RouteTable] {
	return util.FromYaml[*v2.RouteTable](simpleRouteTable)
}
