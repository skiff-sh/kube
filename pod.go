package kube

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"slices"
)

// ContainerStatusToErr if the container is in an error state, it's returned
// with a more human-friendly error.
func ContainerStatusToErr(s *corev1.ContainerStatus) error {
	if s.Ready {
		return nil
	}

	if s.State.Terminated != nil {
		return fmt.Errorf("container %s terminated: %s", s.Name, s.State.Terminated.Message)
	}

	if s.State.Waiting != nil {
		switch s.State.Waiting.Reason {
		case "ImagePullBackOff":
			return fmt.Errorf("container %s has an invalid image: %s", s.Name, s.State.Waiting.Message)
		case "CrashLoopBackoff", "CreateContainerConfigError":
			return fmt.Errorf("container %s failed to start: %s", s.Name, s.State.Waiting.Message)
		}
	}

	return nil
}

// IsPodReady returns true if the pod is ready false otherwise.
func IsPodReady(pod *corev1.Pod) bool {
	if isPodDeleting(pod) {
		return false
	}

	rdyCond := slices.IndexFunc(pod.Status.Conditions, func(condition corev1.PodCondition) bool {
		return condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue
	})
	return rdyCond >= 0
}

// PodErr if the pod is in an unrecoverable state, the associated error is returned.
func PodErr(pod *corev1.Pod) error {
	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodUnknown {
		return errors.New(pod.Status.Reason)
	}

	for _, status := range append(pod.Status.ContainerStatuses, pod.Status.InitContainerStatuses...) {
		err := ContainerStatusToErr(&status)
		if err != nil {
			return err
		}
	}

	return nil
}

func WaitPodReady(ctx context.Context, watcher Watcher, selector labels.Selector) (*corev1.Pod, error) {
	inter, err := watcher.Watch(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	defer inter.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case ev := <-inter.ResultChan():
			pod, _ := ev.Object.(*corev1.Pod)
			if err := PodErr(pod); err != nil {
				return nil, err
			}
			if IsPodReady(pod) {
				return pod, nil
			}
		}
	}
}

func LogFirstPod(ctx context.Context, kc LogGetLister[*corev1.PodList], lo metav1.ListOptions, logOpts corev1.PodLogOptions) (*corev1.Pod, string, error) {
	pod, err := ListFirstPod(ctx, kc, lo)
	if err != nil {
		return nil, "", err
	}

	logs, err := LogPod(ctx, kc, pod.Name, logOpts)
	if err != nil {
		return nil, "", err
	}

	return pod, logs, err
}

func LogPod(ctx context.Context, kc LogGetter, podName string, o corev1.PodLogOptions) (string, error) {
	req := kc.GetLogs(podName, &o)

	stream, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer func(stream io.ReadCloser) {
		_ = stream.Close()
	}(stream)

	buf := bytes.NewBuffer([]byte{})
	_, err = io.Copy(buf, stream)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ListFirstPod lists all the pods via lister, sorts them via RankPods and grabs the highest
// ranked pod.
func ListFirstPod(ctx context.Context, lister Lister[*corev1.PodList], opts metav1.ListOptions) (*corev1.Pod, error) {
	podList, err := lister.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	if len(podList.Items) == 0 {
		return nil, errors.New("not found")
	}

	return GetFirstPodVal(podList.Items), nil
}

// GetFirstPod sorts the pods by RankPods and grabs the top ranked pod.
func GetFirstPod(pods []*corev1.Pod) *corev1.Pod {
	slices.SortFunc(pods, RankPods)
	return pods[len(pods)-1]
}

// GetFirstPodVal same as GetFirstPod but for values.
func GetFirstPodVal(pods []corev1.Pod) *corev1.Pod {
	slices.SortFunc(pods, RankPodsVal)
	return &pods[len(pods)-1]
}

// RankPodsVal same as RankPods but takes a Pod value rather than a ptr.
func RankPodsVal(a, b corev1.Pod) int {
	return RankPods(&a, &b)
}

// RankPods taken from the podutils package but modified for cmp.Compare.
func RankPods(a, b *corev1.Pod) int {
	// 1. Unassigned < assigned
	// If only one of the pods is unassigned, the unassigned one is smaller
	if a.Spec.NodeName != b.Spec.NodeName {
		if len(a.Spec.NodeName) == 0 {
			return -1
		} else if len(b.Spec.NodeName) == 0 {
			return 1
		}
	}

	// 2. PodPending < PodUnknown < PodRunning
	m := map[corev1.PodPhase]int{corev1.PodPending: 0, corev1.PodUnknown: 1, corev1.PodRunning: 2}
	if m[a.Status.Phase] != m[b.Status.Phase] {
		if m[a.Status.Phase] < m[b.Status.Phase] {
			return -1
		} else {
			return 1
		}
	}

	// 3. Not ready < ready
	// If only one of the pods is not ready, the not ready one is smaller
	aRdy, bRdy := IsPodReady(a), IsPodReady(b)
	if aRdy != bRdy {
		if bRdy {
			return -1
		} else {
			return 1
		}
	}

	// 4. Deleting < Not deleting
	aDeleting, bDeleting := isPodDeleting(a), isPodDeleting(b)
	if aDeleting != bDeleting {
		if aDeleting {
			return -1
		} else {
			return 1
		}
	}

	// 5. Older deletion timestamp < newer deletion timestamp
	if isPodDeleting(a) && isPodDeleting(b) && !a.ObjectMeta.DeletionTimestamp.Equal(b.ObjectMeta.DeletionTimestamp) {
		if a.ObjectMeta.DeletionTimestamp.Before(b.ObjectMeta.DeletionTimestamp) {
			return -1
		} else {
			return 1
		}
	}

	// 6. Been ready for empty time < less time < more time
	// If both pods are ready, the latest ready one is smaller
	if aRdy && bRdy && !podReadyTime(a).Equal(podReadyTime(b)) {
		if podReadyTime(a).After(podReadyTime(b).Time) {
			return -1
		} else {
			return 1
		}
	}

	// 7. Pods with containers with higher restart counts < lower restart counts
	if res := compareMaxContainerRestarts(a, b); res != nil {
		if *res {
			return -1
		} else {
			return 1
		}
	}

	// 8. Empty creation time pods < newer pods < older pods
	if !a.CreationTimestamp.Equal(&b.CreationTimestamp) {
		if a.CreationTimestamp.After(b.CreationTimestamp.Time) {
			return -1
		} else {
			return 1
		}
	}

	return 0
}

func isPodDeleting(pod *corev1.Pod) bool {
	return pod.DeletionTimestamp != nil
}

// We use *bool here to determine equality:
// true: pi has a higher container restart count.
// false: pj has a higher container restart count.
// nil: Both have the same container restart count.
func compareMaxContainerRestarts(pi *corev1.Pod, pj *corev1.Pod) *bool {
	regularRestartsI, sidecarRestartsI := maxContainerRestarts(pi)
	regularRestartsJ, sidecarRestartsJ := maxContainerRestarts(pj)
	if regularRestartsI != regularRestartsJ {
		res := regularRestartsI > regularRestartsJ
		return &res
	}
	// If pods have the same restart count, an attempt is made to compare the restart counts of sidecar containers.
	if sidecarRestartsI != sidecarRestartsJ {
		res := sidecarRestartsI > sidecarRestartsJ
		return &res
	}
	return nil
}

func maxContainerRestarts(pod *corev1.Pod) (regularRestarts, sidecarRestarts int) {
	for _, c := range pod.Status.ContainerStatuses {
		regularRestarts = max(regularRestarts, int(c.RestartCount))
	}
	names := sets.New[string]()
	for _, c := range pod.Spec.InitContainers {
		if c.RestartPolicy != nil && *c.RestartPolicy == corev1.ContainerRestartPolicyAlways {
			names.Insert(c.Name)
		}
	}
	for _, c := range pod.Status.InitContainerStatuses {
		if names.Has(c.Name) {
			sidecarRestarts = max(sidecarRestarts, int(c.RestartCount))
		}
	}
	return
}

func podReadyTime(pod *corev1.Pod) *metav1.Time {
	for _, c := range pod.Status.Conditions {
		// we only care about pod ready conditions
		if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
			return &c.LastTransitionTime
		}
	}
	return &metav1.Time{}
}
