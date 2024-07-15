package plugin

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	solov2 "github.com/solo-io/solo-apis/client-go/common.gloo.solo.io/v2"
	networkingv2 "github.com/solo-io/solo-apis/client-go/networking.gloo.solo.io/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

func (r *RpcPlugin) setHashForCanaryStrategy(ctx context.Context, glooPluginConfig *GlooPlatformAPITrafficRouting, rollout *v1alpha1.Rollout, canaryHash, stableHash string) error {
	selectedRouteTables, err := r.getSelectedRouteTables(ctx, glooPluginConfig)
	if err != nil {
		return err
	}

	for _, routeTable := range selectedRouteTables {
		beforeRouteTable := &networkingv2.RouteTable{}
		routeTable.DeepCopyInto(beforeRouteTable)

		// HTTP Routes: Traverse and modify weight on Stable and Canary Routes
		for _, httpRoute := range routeTable.Spec.GetHttp() {
			forwardTo := httpRoute.GetForwardTo()
			if forwardTo == nil {
				continue
			}

			if !glooPluginConfig.IsHttpRouteSelected(httpRoute) {
				continue
			}

			stableDestination := &solov2.DestinationReference{}
			canaryDestination := &solov2.DestinationReference{}
			for _, destination := range forwardTo.GetDestinations() {
				if DoesDestinationReferenceMatchStableService(destination, rollout) {
					r.LogCtx.Infof("Setting stable pod hash to %s", stableHash)
					destination.Subset[v1alpha1.DefaultRolloutUniqueLabelKey] = stableHash
					stableDestination = destination
				}

				if DoesDestinationReferenceMatchCanaryService(destination, rollout) {
					r.LogCtx.Infof("Setting canary pod hash to %s", canaryHash)
					destination.Subset[v1alpha1.DefaultRolloutUniqueLabelKey] = canaryHash
					canaryDestination = destination
				}

				// We found both the destinations, break early
				if stableDestination != nil && canaryDestination != nil {
					break
				}
			}

			// If a Canary destination hasn't been found, we should create one
			if canaryDestination == nil {
				canaryDestination, err = r.createCanaryDestination(stableDestination, rollout)
				canaryDestination.Subset[v1alpha1.DefaultRolloutUniqueLabelKey] = canaryHash
				forwardTo.Destinations = append(forwardTo.GetDestinations(), canaryDestination)
			}
		}

		// Patch Route Table so that new weights take effect
		if err = r.Client.RouteTables().PatchRouteTable(ctx, routeTable, client.MergeFrom(beforeRouteTable)); err != nil {
			// If we're unable to patch a Route Table, but other Route Tables are selected
			// we collect errors and return all at the end
			// TODO We should attempt to patch the remaining Route Tables before returning an error
			return pluginTypes.RpcError{ErrorString: fmt.Sprintf("failed to patch RouteTable: %s", err)}
		}
	}

	return nil
}

func (r *RpcPlugin) setWeightForCanaryStrategy(ctx context.Context, rollout *v1alpha1.Rollout, desiredWeight int32, glooPluginConfig *GlooPlatformAPITrafficRouting) error {
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
		for _, httpRoute := range routeTable.Spec.GetHttp() {
			forwardTo := httpRoute.GetForwardTo()
			if forwardTo == nil {
				continue
			}

			if !glooPluginConfig.IsHttpRouteSelected(httpRoute) {
				continue
			}

			stableDestination := &solov2.DestinationReference{}
			canaryDestination := &solov2.DestinationReference{}
			for _, destination := range forwardTo.GetDestinations() {
				if DoesDestinationReferenceMatchStableService(destination, rollout) {
					destination.Weight = uint32(remainingWeight)
					stableDestination = destination
				}

				if DoesDestinationReferenceMatchCanaryService(destination, rollout) {
					// The desiredWeight is the weight of the canary installation
					destination.Weight = uint32(desiredWeight)
					canaryDestination = destination
				}

				// We found both the destinations, break early
				if stableDestination != nil && canaryDestination != nil {
					break
				}
			}

			// If a Canary destination hasn't been found, we should create one
			if canaryDestination == nil {
				r.LogCtx.Debugf("Creating a new canary destination")
				canaryDestination, err = r.createCanaryDestination(stableDestination, rollout)
				forwardTo.Destinations = append(forwardTo.GetDestinations(), canaryDestination)
			}
		}

		// Patch Route Table so that new weights take effect
		if err = r.Client.RouteTables().PatchRouteTable(ctx, routeTable, client.MergeFrom(beforeRouteTable)); err != nil {
			// If we're unable to patch a Route Table, but other Route Tables are selected
			// we collect errors and return all at the end
			// TODO We should attempt to patch the remaining Route Tables before returning an error
			return pluginTypes.RpcError{ErrorString: fmt.Sprintf("failed to patch RouteTable: %s", err)}
		}
	}

	return nil
}

func (r *RpcPlugin) createCanaryDestination(stableDest *solov2.DestinationReference, rollout *v1alpha1.Rollout) (*solov2.DestinationReference, error) {
	newDest := stableDest.Clone().(*solov2.DestinationReference)
	newDest.GetRef().Name = rollout.Spec.Strategy.Canary.CanaryService
	return newDest, nil
}

func DoesDestinationReferenceMatchStableService(destinationReference *solov2.DestinationReference, rollout *v1alpha1.Rollout) bool {
	return strings.EqualFold(destinationReference.GetRef().GetName(), rollout.Spec.Strategy.Canary.StableService)
}

func DoesDestinationReferenceMatchCanaryService(destinationReference *solov2.DestinationReference, rollout *v1alpha1.Rollout) bool {
	return strings.EqualFold(destinationReference.GetRef().GetName(), rollout.Spec.Strategy.Canary.CanaryService)
}
