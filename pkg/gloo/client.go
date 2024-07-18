package gloo

import (
	"context"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform/pkg/util"

	networkv2 "github.com/solo-io/solo-apis/client-go/networking.gloo.solo.io/v2"
	"k8s.io/apimachinery/pkg/runtime"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

//go:generate mockgen -source ./client.go -destination mocks/client.go

type DumbObjectSelector struct {
	Labels    map[string]string `json:"labels" protobuf:"bytes,1,name=labels"`
	Name      string            `json:"name" protobuf:"bytes,2,name=name"`
	Namespace string            `json:"namespace" protobuf:"bytes,3,name=namespace"`
}

func (d *DumbObjectSelector) IsNameEmpty() bool {
	return strings.EqualFold(d.Name, "")
}

type networkV2Client struct {
	routeTableClient *routeTableClient
}

func (c networkV2Client) PatchRouteTable(ctx context.Context, original *networkv2.RouteTable, toPatch *networkv2.RouteTable, opts ...k8sclient.PatchOption) error {
	return c.RouteTables().PatchRouteTable(ctx, original, k8sclient.MergeFrom(toPatch), opts...)
}

type NetworkV2ClientSet interface {
	RouteTables() RouteTableClient
	PatchRouteTable(ctx context.Context, original *networkv2.RouteTable, toPatch *networkv2.RouteTable, opts ...k8sclient.PatchOption) error
}

type RouteTableClient interface {
	RouteTableReader
	RouteTableWriter
}

type RouteTableReader interface {
	// Get retrieves a RouteTable for the given object key
	GetRouteTable(ctx context.Context, name string, namespace string) (*networkv2.RouteTable, error)

	// List retrieves list of RouteTables for a given namespace and list options.
	ListRouteTable(ctx context.Context, opts ...k8sclient.ListOption) ([]*networkv2.RouteTable, error)
}

type RouteTableWriter interface {
	// Patch patches the given RouteTable object.
	PatchRouteTable(ctx context.Context, obj *networkv2.RouteTable, patch k8sclient.Patch, opts ...k8sclient.PatchOption) error
}

type routeTableClient struct {
	client k8sclient.Client
}

func NewNetworkV2ClientSet() (NetworkV2ClientSet, error) {
	cfg, err := util.GetKubeConfig()
	if err != nil {
		return nil, err
	}

	scheme := runtime.NewScheme()
	networkv2.AddToScheme(scheme)
	c, err := k8sclient.New(cfg, k8sclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}

	return networkV2Client{
		routeTableClient: &routeTableClient{client: c},
	}, nil
}

func (c networkV2Client) RouteTables() RouteTableClient {
	return c.routeTableClient
}
