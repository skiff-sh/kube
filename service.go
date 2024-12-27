package kube

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"slices"
)

func IndexPortForService(sp corev1.ServicePort, cons []corev1.Container) (conIdx int, portIdx int) {
	var matcher func(p corev1.ContainerPort) bool
	if sp.TargetPort.Type == intstr.Int {
		matcher = func(p corev1.ContainerPort) bool {
			return p.ContainerPort == sp.TargetPort.IntVal
		}
	} else {
		matcher = func(p corev1.ContainerPort) bool {
			return p.Name == sp.TargetPort.StrVal
		}
	}

	return IndexContainerPort(cons, matcher)
}

func IndexContainerPort(cons []corev1.Container, matcher func(c corev1.ContainerPort) bool) (conIdx int, portIdx int) {
	conIdx = slices.IndexFunc(cons, func(container corev1.Container) bool {
		portIdx = slices.IndexFunc(container.Ports, matcher)
		return portIdx >= 0
	})
	return
}
