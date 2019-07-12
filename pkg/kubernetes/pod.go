package kubernetes

import (
	"fmt"
	"k8s.io/api/core/v1"
)

type PodInfo struct {
	ResourceVersion string
	name            string
	namespace       string
	PodIP           string
	HostIP          string
	HostNetwork     bool
	Labels          map[string]string
}

func (pod *PodInfo) GetSelector() map[string]string {
	return pod.Labels
}

func (pod *PodInfo) Type() ResourceType {
	return POD_TYPE
}

func (pod *PodInfo) Name() string {
	return pod.name
}

func (pod *PodInfo) Namespace() string {
	return pod.namespace
}

func (pod *PodInfo) String() string {
	return fmt.Sprintf("Pod %s@%s", pod.name, pod.namespace)
}

func (pod *PodInfo) IsSkip() bool {
	return pod.namespace == "kube-system"
}

func NewPodInfo(pod *v1.Pod) *PodInfo {
	if pod.Status.PodIP == "" {
		return nil
	}

	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	return &PodInfo{
		PodIP:           pod.Status.PodIP,
		HostIP:          pod.Status.HostIP,
		namespace:       pod.Namespace,
		name:            pod.Name,
		Labels:          pod.Labels,
		HostNetwork:     pod.Spec.HostNetwork,
		ResourceVersion: pod.ResourceVersion,
	}
}
