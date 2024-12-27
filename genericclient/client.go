package genericclient

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type GenericClient interface {
	Create(ctx context.Context, raw []byte, o metav1.CreateOptions) ([]byte, error)
	Get(ctx context.Context, name string, o metav1.GetOptions) ([]byte, error)
	Update(ctx context.Context, raw []byte, o metav1.UpdateOptions) ([]byte, error)
	Delete(ctx context.Context, name string, o metav1.DeleteOptions) error
}

type ResourceClient interface {
	GroupVersionKind() schema.GroupVersionKind
	Resource() string
	IsClusterLevel() bool
	Immutable() bool
}

type genericClient struct {
}
