package config

import (
	"encoding/json"
	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform/pkg/gloo"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	networkv2 "github.com/solo-io/solo-apis/client-go/networking.gloo.solo.io/v2"
	"strings"
)

const (
	PluginName = "solo-io/glooplatform"
)

type GlooPlatformAPITrafficRouting struct {
	RouteTableSelector *gloo.DumbObjectSelector `json:"routeTableSelector" protobuf:"bytes,1,name=routeTableSelector"`
	RouteSelector      *gloo.DumbObjectSelector `json:"routeSelector" protobuf:"bytes,2,name=routeSelector"`

	// Allows the selection of subsets in a Route Table
	// Subset Selector will
	CanarySubsetSelector map[string]string `json:"canarySubsetSelector" protobuf:"bytes,3,name=canarySubsetSelector"`
	StableSubsetSelector map[string]string `json:"stableSubsetSelector" protobuf:"bytes,4,name=stableSubsetSelector"`
}

func GetPluginConfiguration(rollout *v1alpha1.Rollout) (*GlooPlatformAPITrafficRouting, error) {
	glooPlatformConfig := GlooPlatformAPITrafficRouting{}

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

func (g *GlooPlatformAPITrafficRouting) IsRouteSelectorDefined() bool {
	return g.RouteSelector != nil
}

func (g *GlooPlatformAPITrafficRouting) IsHttpRouteSelected(httpRoute *networkv2.HTTPRoute) bool {
	if !g.IsRouteSelectorDefined() {
		return true
	}

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

	return false
}

type DumbObjectSelector struct {
	Labels    map[string]string `json:"labels" protobuf:"bytes,1,name=labels"`
	Name      string            `json:"name" protobuf:"bytes,2,name=name"`
	Namespace string            `json:"namespace" protobuf:"bytes,3,name=namespace"`
}

func (d *DumbObjectSelector) IsNameEmpty() bool {
	return strings.EqualFold(d.Name, "")
}
