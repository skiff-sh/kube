package genericclient

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"strings"
)

type ResourceClientOpts struct {
	Immutable    bool
	ClusterLevel bool
}

func NewResourceClient(gvk schema.GroupVersionKind, cl rest.Interface, op ResourceClientOpts) ResourceClient {
	out := &resourceClient{
		Res:          strings.ToLower(gvk.Kind) + "s",
		Client:       cl,
		GVK:          gvk,
		ClusterLevel: op.ClusterLevel,
		IsImmutable:  op.Immutable,
	}

	return out
}

type resourceClient struct {
	Res          string
	GVK          schema.GroupVersionKind
	Client       rest.Interface
	ClusterLevel bool
	IsImmutable  bool
}

func (r *resourceClient) Immutable() bool {
	return r.IsImmutable
}

func (r *resourceClient) GroupVersionKind() schema.GroupVersionKind {
	return r.GVK
}

func (r *resourceClient) IsClusterLevel() bool {
	return r.ClusterLevel
}

func (r *resourceClient) Resource() string {
	return r.Res
}

func (r *resourceClient) RESTClient() rest.Interface {
	return r.Client
}
