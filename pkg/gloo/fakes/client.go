package fakes

import (
	"context"
	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform/pkg/gloo"
	v2 "github.com/solo-io/solo-apis/client-go/networking.gloo.solo.io/v2"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type FakeRouteTableClient struct {
	ListRouteTableFunc  func(ctx context.Context, opts ...k8sclient.ListOption) ([]*v2.RouteTable, error)
	PatchRouteTableFunc func(ctx context.Context, obj *v2.RouteTable, patch k8sclient.Patch, opts ...k8sclient.PatchOption) error
	GetRouteTableFunc   func(ctx context.Context, name, namespace string) (*v2.RouteTable, error)
}

func (f FakeRouteTableClient) PatchRouteTable(ctx context.Context, obj *v2.RouteTable, patch k8sclient.Patch, opts ...k8sclient.PatchOption) error {
	return f.PatchRouteTableFunc(ctx, obj, patch, opts...)
}

func (f FakeRouteTableClient) GetRouteTable(ctx context.Context, name string, namespace string) (*v2.RouteTable, error) {
	return f.GetRouteTableFunc(ctx, name, namespace)
}

func (f FakeRouteTableClient) ListRouteTable(ctx context.Context, opts ...k8sclient.ListOption) ([]*v2.RouteTable, error) {
	return f.ListRouteTableFunc(ctx, opts...)
}

type FakeClientSet struct {
	PatchRouteTableFunc func(ctx context.Context, original *v2.RouteTable, toPatch *v2.RouteTable, opts ...k8sclient.PatchOption) error
	RouteTableFunc      func() gloo.RouteTableClient
}

func (f FakeClientSet) RouteTables() gloo.RouteTableClient {
	return f.RouteTableFunc()
}

func (f FakeClientSet) PatchRouteTable(ctx context.Context, patch *v2.RouteTable, before *v2.RouteTable, opts ...k8sclient.PatchOption) error {
	return f.PatchRouteTableFunc(ctx, patch, before, opts...)
}
