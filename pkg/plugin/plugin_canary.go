package plugin

import (
	"context"
	"fmt"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform/pkg/plugin/config"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	solov2 "github.com/solo-io/solo-apis/client-go/common.gloo.solo.io/v2"
	networkingv2 "github.com/solo-io/solo-apis/client-go/networking.gloo.solo.io/v2"
	"strings"
)

func (r *RpcPlugin) setWeightForCanaryStrategy(ctx context.Context, rollout *v1alpha1.Rollout, desiredWeight int32, glooPluginConfig *config.GlooPlatformAPITrafficRouting) error {
	// The desiredWeight is the weight to be assigned to Canary
	// The remainingWeight is whatever is left over for Stable
	remainingWeight := 100 - desiredWeight

	selectedRouteTables, err := r.getSelectedRouteTables(ctx, glooPluginConfig)
	if err != nil {
		return err
	}

	for _, routeTable := range selectedRouteTables {
		beforeRouteTable := &networkingv2.RouteTable{}
		routeTable.DeepCopyInto(beforeRouteTable)

		// HTTP Routes: Traverse and modify weight on Stable and Canary Routes
		for httpRouteIndex, httpRoute := range routeTable.Spec.GetHttp() {
			forwardTo := httpRoute.GetForwardTo()
			if forwardTo == nil {
				continue
			}

			if !glooPluginConfig.IsHttpRouteSelected(httpRoute) {
				continue
			}

			var stableDestination *solov2.DestinationReference
			var canaryDestination *solov2.DestinationReference
			for destinationIndex, destination := range forwardTo.GetDestinations() {
				if DoesDestinationReferenceMatchStableService(destination, rollout) {
					destination.Weight = uint32(remainingWeight)
					stableDestination = destination
					forwardTo.GetDestinations()[destinationIndex] = destination
				}

				if DoesDestinationReferenceMatchCanaryService(destination, rollout) {
					// The desiredWeight is the weight of the canary installation
					destination.Weight = uint32(desiredWeight)
					canaryDestination = destination
					forwardTo.GetDestinations()[destinationIndex] = destination
				}

				// We found both the destinations, break early
				if stableDestination != nil && canaryDestination != nil {
					break
				}
			}

			// If a Canary destination hasn't been found, we should create one
			if canaryDestination == nil {
				r.LogCtx.Debugf("Creating a new canary destination for RouteTable %s.%s", routeTable.GetName(), routeTable.GetNamespace())
				canaryDestination, err = r.createCanaryDestination(stableDestination, rollout, desiredWeight)
				forwardTo.Destinations = append(forwardTo.GetDestinations(), canaryDestination)
			}
			//
			routeTable.Spec.GetHttp()[httpRouteIndex].ActionType.(*networkingv2.HTTPRoute_ForwardTo).ForwardTo = forwardTo
		}

		// Patch Route Table so that new weights take effect
		if err = r.Client.PatchRouteTable(ctx, routeTable, beforeRouteTable); err != nil {
			// If we're unable to patch a Route Table, but other Route Tables are selected
			// we collect errors and return all at the end
			// TODO We should attempt to patch the remaining Route Tables before returning an error
			return pluginTypes.RpcError{ErrorString: fmt.Sprintf("failed to patch RouteTable: %s", err)}
		}
	}

	return nil
}

func (r *RpcPlugin) createCanaryDestination(stableDest *solov2.DestinationReference, rollout *v1alpha1.Rollout, desiredWeight int32) (*solov2.DestinationReference, error) {
	newDest := stableDest.Clone().(*solov2.DestinationReference)
	newDest.GetRef().Name = rollout.Spec.Strategy.Canary.CanaryService
	newDest.Weight = uint32(desiredWeight)
	return newDest, nil
}

func DoesDestinationReferenceMatchStableService(destinationReference *solov2.DestinationReference, rollout *v1alpha1.Rollout) bool {
	// TODO is destination kind SERVICE
	return strings.EqualFold(destinationReference.GetRef().GetName(), rollout.Spec.Strategy.Canary.StableService)
}

func DoesDestinationReferenceMatchCanaryService(destinationReference *solov2.DestinationReference, rollout *v1alpha1.Rollout) bool {
	// TODO is destination kind SERVICE
	return strings.EqualFold(destinationReference.GetRef().GetName(), rollout.Spec.Strategy.Canary.CanaryService)
}
