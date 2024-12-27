package kube

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

// CoreV1Client is an interface to represent the kubernetes.Interface without
// having to import the entire go client.
type CoreV1Client interface {
	Services(namespace string) Client[*corev1.Service, *corev1.ServiceList]
	Pods(namespace string) Client[*corev1.Pod, *corev1.PodList]
}

type ObjectType interface {
	runtime.Object
	metav1.Object
}

type ListType interface {
	runtime.Object
	meta.List
}

type Creator[T ObjectType] interface {
	Create(ctx context.Context, obj T, opts metav1.CreateOptions) (T, error)
}

type Updater[T ObjectType] interface {
	Update(ctx context.Context, obj T, opts metav1.UpdateOptions) (T, error)
}

type Getter[T ObjectType] interface {
	Get(ctx context.Context, name string, opts metav1.GetOptions) (T, error)
}

type Deleter interface {
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
}

type Watcher interface {
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
}

type Lister[T ListType] interface {
	List(ctx context.Context, opts metav1.ListOptions) (T, error)
}

type InformerLister[T ObjectType] interface {
	List(selector labels.Selector) (ret []T, err error)
}

type InformerGetter[T ObjectType] interface {
	Get(name string) (T, error)
}

type CollectionDeleter interface {
	DeleteCollection(ctx context.Context, do metav1.DeleteOptions, lo metav1.ListOptions) error
}

type LogGetLister[T ListType] interface {
	LogGetter
	Lister[T]
}

type LogGetter interface {
	GetLogs(name string, lo *corev1.PodLogOptions) *rest.Request
}

type Client[T ObjectType, L ListType] interface {
	ReadClient[T, L]
	Creator[T]
	Updater[T]
	Deleter
}

type ReadClient[T ObjectType, L ListType] interface {
	Getter[T]
	Lister[L]
	Watcher
}
