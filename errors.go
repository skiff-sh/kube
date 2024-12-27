package kube

import (
	"errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ErrHasReason checks if the error has any of the listed reasons.
func ErrHasReason(err error, reasons ...metav1.StatusReason) bool {
	reason := ErrReason(err)
	for _, v := range reasons {
		if v == reason {
			return true
		}
	}
	return false
}

// ErrReason extracts the reason from the error if it's from k8s.
func ErrReason(err error) metav1.StatusReason {
	var v *apierrors.StatusError
	if errors.As(err, &v) {
		return v.ErrStatus.Reason
	}

	return ""
}
