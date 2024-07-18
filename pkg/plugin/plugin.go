package plugin

import (
	"context"
	"fmt"
	"strings"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform/pkg/gloo"
	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform/pkg/plugin/config"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/argoproj/argo-rollouts/rollout/trafficrouting/plugin/rpc"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	"github.com/sirupsen/logrus"
	networkv2 "github.com/solo-io/solo-apis/client-go/networking.gloo.solo.io/v2"
	"k8s.io/apimachinery/pkg/labels"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	Type                       = "GlooPlatformAPI"
	GlooPlatformAPIUpdateError = "GlooPlatformAPIUpdateError"
)

type RpcPlugin struct {
	// todo REMOVE
	IsTest bool

	LogCtx *logrus.Entry
	Client gloo.NetworkV2ClientSet
}

func NewRPCPlugin(
	logCtx *logrus.Entry,
	client gloo.NetworkV2ClientSet,
) rpc.TrafficRouterPlugin {
	return &RpcPlugin{
		IsTest: false,
		LogCtx: logCtx,
		Client: client,
	}
}

func (r *RpcPlugin) InitPlugin() pluginTypes.RpcError {
	if r.IsTest {
		return pluginTypes.RpcError{}
	}
	client, err := gloo.NewNetworkV2ClientSet()
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	r.Client = client
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) UpdateHash(rollout *v1alpha1.Rollout, canaryHash, stableHash string, additionalDestinations []v1alpha1.WeightDestination) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination) pluginTypes.RpcError {
	ctx := context.TODO()
	glooPluginConfig, err := config.GetPluginConfiguration(rollout)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}

	if rollout.Spec.Strategy.Canary != nil && IsServiceTarget(rollout) {
		r.LogCtx.Debugf("For canary strategy, setting weight to %d", desiredWeight)
		if err = r.setWeightForCanaryStrategy(ctx, rollout, desiredWeight, glooPluginConfig); err != nil {
			return pluginTypes.RpcError{ErrorString: err.Error()}
		}
	} else if rollout.Spec.Strategy.BlueGreen != nil {
		return r.handleBlueGreen(rollout)
	}

	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetHeaderRoute(rollout *v1alpha1.Rollout, headerRouting *v1alpha1.SetHeaderRoute) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetMirrorRoute(rollout *v1alpha1.Rollout, setMirrorRoute *v1alpha1.SetMirrorRoute) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) VerifyWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination) (pluginTypes.RpcVerified, pluginTypes.RpcError) {
	return pluginTypes.Verified, pluginTypes.RpcError{}
}

func (r *RpcPlugin) RemoveManagedRoutes(rollout *v1alpha1.Rollout) pluginTypes.RpcError {
	// we could remove the canary destination, but not required since it will have 0 weight at the end of rollout
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) Type() string {
	return Type
}

func (r *RpcPlugin) getSelectedRouteTables(ctx context.Context, glooPluginConfig *config.GlooPlatformAPITrafficRouting) ([]*networkv2.RouteTable, error) {
	if glooPluginConfig.RouteTableSelector == nil {
		return nil, fmt.Errorf("routeTable selector is required")
	}

	var rts []*networkv2.RouteTable
	// If name is not empty, we're getting a specific route table
	if !glooPluginConfig.RouteTableSelector.IsNameEmpty() {
		r.LogCtx.Debugf("getRouteTables using ns:name ref %s:%s to get single table", glooPluginConfig.RouteTableSelector.Name, glooPluginConfig.RouteTableSelector.Namespace)
		result, err := r.Client.RouteTables().GetRouteTable(ctx, glooPluginConfig.RouteTableSelector.Name, glooPluginConfig.RouteTableSelector.Namespace)
		if err != nil {
			return nil, err
		}

		r.LogCtx.Debugf("getRouteTables using ns:name ref %s:%s found 1 table", glooPluginConfig.RouteTableSelector.Name, glooPluginConfig.RouteTableSelector.Namespace)
		rts = append(rts, result)
	} else {
		// Get Route Table by Labels
		opts := &k8sclient.ListOptions{}

		if glooPluginConfig.RouteTableSelector.Labels != nil {
			opts.LabelSelector = labels.SelectorFromSet(glooPluginConfig.RouteTableSelector.Labels)
		}
		if !strings.EqualFold(glooPluginConfig.RouteTableSelector.Namespace, "") {
			opts.Namespace = glooPluginConfig.RouteTableSelector.Namespace
		}

		r.LogCtx.Debugf("heh getRouteTables listing tables with opts %+v", opts)
		var err error

		rts, err = r.Client.RouteTables().ListRouteTable(ctx, opts)
		if err != nil {
			return nil, err
		}
		r.LogCtx.Debugf("heh getRouteTables listing tables with opts %+v; found %d routeTables", opts, len(rts))
	}
	return rts, nil
}

func IsServiceTarget(rollout *v1alpha1.Rollout) bool {
	return rollout.Spec.Strategy.Canary.StableService != "" && rollout.Spec.Strategy.Canary.CanaryService != ""
}
