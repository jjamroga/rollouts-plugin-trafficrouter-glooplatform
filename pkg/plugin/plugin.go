package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform/pkg/gloo"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	"github.com/sirupsen/logrus"
	networkv2 "github.com/solo-io/solo-apis/client-go/networking.gloo.solo.io/v2"
	"k8s.io/apimachinery/pkg/labels"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	Type                       = "GlooPlatformAPI"
	GlooPlatformAPIUpdateError = "GlooPlatformAPIUpdateError"
	PluginName                 = "solo-io/glooplatformdev"
)

type RpcPlugin struct {
	// todo REMOVE
	IsTest bool
	// temporary hack until mock clienset is fixed (missing some interface methods)
	// TestRouteTable *networkv2.RouteTable
	LogCtx *logrus.Entry
	Client gloo.NetworkV2ClientSet
}

type GlooPlatformAPITrafficRouting struct {
	RouteTableSelector *gloo.DumbObjectSelector `json:"routeTableSelector" protobuf:"bytes,1,name=routeTableSelector"`
	RouteSelector      *gloo.DumbObjectSelector `json:"routeSelector" protobuf:"bytes,2,name=routeSelector"`

	// Allows the selection of subsets in a Route Table
	// Subset Selector will
	CanarySubsetSelector map[string]string `json:"canarySubsetSelector" protobuf:"bytes,3,name=canarySubsetSelector"`
	StableSubsetSelector map[string]string `json:"stableSubsetSelector" protobuf:"bytes,4,name=stableSubsetSelector"`
}

func (g *GlooPlatformAPITrafficRouting) IsRouteSelectorDefined() bool {
	return g.RouteSelector != nil
}

func (g *GlooPlatformAPITrafficRouting) IsHttpRouteSelected(httpRoute *networkv2.HTTPRoute) bool {
	if g.RouteSelector != nil {
		// if name was provided, skip if route name doesn't match
		if !strings.EqualFold(g.RouteSelector.Name, "") && !strings.EqualFold(g.RouteSelector.Name, httpRoute.Name) {
			// logCtx.Debugf("skipping route %s.%s because it doesn't match route name selector %s", routeTable.Name, httpRoute.Name, trafficConfig.RouteSelector.Name)
			return false
		}
		// if labels provided, skip if route labels do not contain all specified labels
		if g.RouteSelector.Labels != nil {
			matchedLabels := func() bool {
				for k, v := range g.RouteSelector.Labels {
					if vv, ok := httpRoute.Labels[k]; ok {
						if !strings.EqualFold(v, vv) {
							// logCtx.Debugf("skipping route %s.%s because route labels do not contain %s=%s", routeTable.Name, routeTable.Name, k, v)
							return false
						}
					}
				}
				return true
			}()
			if !matchedLabels {
				return false
			}
		}
		// logCtx.Debugf("route %s.%s passed RouteSelector", g.RouteTable.Name, httpRoute.Name)
	}

	return true
}

type DumbRouteSelector struct {
	Labels map[string]string `json:"labels" protobuf:"bytes,1,name=labels"`
	Name   string            `json:"name" protobuf:"bytes,2,name=name"`
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
	r.LogCtx.Debugf("SETTING POD HASH stable: %s canary %s", stableHash, canaryHash)
	// Inject the RouteTable selectors with the pod hash
	// SetWeight and UpdateHash will follow the same process, but what will be different is the action
	// SetWeight will modify the weight on DestinationReference
	// UpdateHash will add a PodHash to the subset label
	//ctx := context.TODO()
	//glooPluginConfig, err := getPluginConfig(rollout)
	//if err != nil {
	//	return pluginTypes.RpcError{
	//		ErrorString: err.Error(),
	//	}
	//}
	//
	//if rollout.Spec.Strategy.Canary != nil {
	//	if err = r.handleCanarySubsetHash(ctx, rollout, glooPluginConfig); err != nil {
	//		return pluginTypes.RpcError{ErrorString: err.Error()}
	//	}
	//} else if rollout.Spec.Strategy.BlueGreen != nil {
	//	return pluginTypes.RpcError{}
	//}
	return pluginTypes.RpcError{
		ErrorString: "Is this even getting called?",
	}
	//ctx := context.TODO()
	//glooPluginConfig, err := getPluginConfig(rollout)
	//if err != nil {
	//	return pluginTypes.RpcError{
	//		ErrorString: err.Error(),
	//	}
	//}
	//
	//r.LogCtx.Infof("UpdateHash called on rollout %s.%s", rollout.Name, rollout.Namespace)
	//if rollout.Spec.Strategy.Canary != nil && rollout.Spec.Strategy.Canary.StableService == "" && rollout.Spec.Strategy.Canary.CanaryService == "" {
	//	r.LogCtx.Debug("ATTEMPTING TO SET HASH")
	//	if err = r.setHashForCanaryStrategy(ctx, glooPluginConfig, rollout, canaryHash, stableHash); err != nil {
	//		return pluginTypes.RpcError{ErrorString: err.Error()}
	//	}
	//} else if rollout.Spec.Strategy.BlueGreen != nil {
	//	return r.handleBlueGreen(rollout)
	//}
}

func (r *RpcPlugin) SetWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination) pluginTypes.RpcError {
	ctx := context.TODO()
	glooPluginConfig, err := r.getPluginConfig(rollout)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}

	if rollout.Spec.Strategy.Canary != nil {
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

func (r *RpcPlugin) getPluginConfig(rollout *v1alpha1.Rollout) (*GlooPlatformAPITrafficRouting, error) {
	glooPlatformConfig := GlooPlatformAPITrafficRouting{}

	r.LogCtx.Debugf("Checking for config for plugin %s", PluginName)
	err := json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugins[PluginName], &glooPlatformConfig)
	if err != nil {
		return nil, err
	}

	if strings.EqualFold(glooPlatformConfig.RouteTableSelector.Namespace, "") {
		// If namespace is not specified, we default to the namespace of the rollout
		glooPlatformConfig.RouteTableSelector.Namespace = rollout.Namespace
	}

	return &glooPlatformConfig, nil
}

func (r *RpcPlugin) getSelectedRouteTables(ctx context.Context, glooPluginConfig *GlooPlatformAPITrafficRouting) ([]*networkv2.RouteTable, error) {
	if glooPluginConfig.RouteTableSelector == nil {
		return nil, fmt.Errorf("routeTable selector is required")
	}

	var rts []*networkv2.RouteTable
	// If name is not empty, we're getting a specific route table
	if !glooPluginConfig.RouteTableSelector.IsNameEmpty() {
		r.LogCtx.Debugf("heh getRouteTables using ns:name ref %s:%s to get single table", glooPluginConfig.RouteTableSelector.Name, glooPluginConfig.RouteTableSelector.Namespace)
		result, err := r.Client.RouteTables().GetRouteTable(ctx, glooPluginConfig.RouteTableSelector.Name, glooPluginConfig.RouteTableSelector.Namespace)
		if err != nil {
			return nil, err
		}

		r.LogCtx.Debugf("heh getRouteTables using ns:name ref %s:%s found 1 table", glooPluginConfig.RouteTableSelector.Name, glooPluginConfig.RouteTableSelector.Namespace)
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
