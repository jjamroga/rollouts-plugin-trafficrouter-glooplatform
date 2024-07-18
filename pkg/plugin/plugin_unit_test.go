package plugin

import (
	"context"
	"fmt"
	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform/examples"
	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform/pkg/gloo"
	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform/pkg/gloo/fakes"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/argoproj/argo-rollouts/utils/plugin/types"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	log "github.com/sirupsen/logrus"
	commonv2 "github.com/solo-io/solo-apis/client-go/common.gloo.solo.io/v2"
	networkingv2 "github.com/solo-io/solo-apis/client-go/networking.gloo.solo.io/v2"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

type SetWeightTestCase struct {
	Name             string
	DesiredWeight    int32
	RouteTables      []*networkingv2.RouteTable
	Rollout          *v1alpha1.Rollout
	AssertHttpRoutes map[string]func(g gomega.Gomega, destinations []*commonv2.DestinationReference)
	AssertReturns    types.RpcError
}

func Test_SetWeight(t *testing.T) {
	destinationReferenceId := func(element interface{}) string {
		ref := element.(*commonv2.DestinationReference).GetRef()
		return fmt.Sprintf("%s.%s", ref.GetName(), ref.GetNamespace())
	}

	testCases := []SetWeightTestCase{{
		Name:          "Canary destination will be automatically created on an HTTP route",
		RouteTables:   []*networkingv2.RouteTable{examples.GetSimpleRouteTable().Build()},
		Rollout:       examples.GetSimpleCanaryRollout().Build(),
		DesiredWeight: 0,
		AssertHttpRoutes: map[string]func(g gomega.Gomega, destinations []*commonv2.DestinationReference){
			"demo": func(g gomega.Gomega, destinations []*commonv2.DestinationReference) {
				g.Expect(destinations).To(gstruct.MatchElements(destinationReferenceId, gstruct.IgnoreExtras, gstruct.Elements{
					"canary.gloo-rollouts-demo": gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreMissing|gstruct.IgnoreExtras, gstruct.Fields{
						"Weight": gomega.BeEquivalentTo(0),
					})),
					"stable.gloo-rollouts-demo": gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreMissing|gstruct.IgnoreExtras, gstruct.Fields{
						"Weight": gomega.BeEquivalentTo(100),
					})),
				}))
			},
		},
	}, {
		Name: "An existing canary route will be modified with a weight",
		RouteTables: []*networkingv2.RouteTable{
			examples.GetSimpleRouteTable().Modify(func(obj *networkingv2.RouteTable) {
				obj.Spec.GetHttp()[1] = CreateHttpRouteWithUnusedCanaryDestination(&commonv2.ObjectReference{
					Name:      "stable",
					Namespace: "gloo-rollouts-demo",
				}, &commonv2.ObjectReference{
					Name:      "canary",
					Namespace: "gloo-rollouts-demo",
				})
			}).Build(),
		},
		Rollout:       examples.GetSimpleCanaryRollout().Build(),
		DesiredWeight: 10,
		AssertHttpRoutes: map[string]func(g gomega.Gomega, destinations []*commonv2.DestinationReference){
			"demo": func(g gomega.Gomega, destinations []*commonv2.DestinationReference) {
				g.Expect(destinations).To(gstruct.MatchElements(destinationReferenceId, gstruct.IgnoreExtras, gstruct.Elements{
					"canary.gloo-rollouts-demo": gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreMissing|gstruct.IgnoreExtras, gstruct.Fields{
						"Weight": gomega.BeEquivalentTo(10),
					})),
					"stable.gloo-rollouts-demo": gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreMissing|gstruct.IgnoreExtras, gstruct.Fields{
						"Weight": gomega.BeEquivalentTo(90),
					})),
				}))
			},
		},
	}}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			fakeRouteTableClient := fakes.FakeRouteTableClient{
				// We're not testing that RouteTable retrieval works in this test, just get the Route Table back
				ListRouteTableFunc: func(ctx context.Context, opts ...k8sclient.ListOption) ([]*networkingv2.RouteTable, error) {
					return testCase.RouteTables, nil
				},
			}
			fakeClientSet := fakes.FakeClientSet{
				RouteTableFunc: func() gloo.RouteTableClient {
					return fakeRouteTableClient
				},
			}

			// Expect that the Canary Route has been added to the RouteTable
			fakeClientSet.PatchRouteTableFunc = func(ctx context.Context, patch *networkingv2.RouteTable, before *networkingv2.RouteTable, opts ...k8sclient.PatchOption) error {
				for _, httpRoute := range patch.Spec.GetHttp() {
					assertion, ok := testCase.AssertHttpRoutes[httpRoute.GetName()]
					if ok {
						assertion(g, httpRoute.GetForwardTo().GetDestinations())
					}
				}
				return nil
			}

			plugin := NewRPCPlugin(
				log.WithFields(log.Fields{"plugin": "trafficrouter"}),
				fakeClientSet,
			)
			g.Expect(plugin.SetWeight(testCase.Rollout, testCase.DesiredWeight, nil)).To(gomega.Equal(testCase.AssertReturns))
		})
	}
}

func CreateHttpRouteWithUnusedCanaryDestination(stableRef, canaryRef *commonv2.ObjectReference) *networkingv2.HTTPRoute {
	return &networkingv2.HTTPRoute{
		Name: "demo",
		ActionType: &networkingv2.HTTPRoute_ForwardTo{
			ForwardTo: &networkingv2.ForwardToAction{
				Destinations: []*commonv2.DestinationReference{
					{
						RefKind: &commonv2.DestinationReference_Ref{
							Ref: stableRef,
						},
						Weight: 100,
					},
					{
						RefKind: &commonv2.DestinationReference_Ref{
							Ref: canaryRef,
						},
						Weight: 0,
					},
				},
			},
		},
	}
}
