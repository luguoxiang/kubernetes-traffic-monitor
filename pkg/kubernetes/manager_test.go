package kubernetes

import (
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	"sync"
	"testing"
)

func TestWatch(t *testing.T) {
	k8sManager := &K8sResourceManager{
		clientSet:            fake.NewSimpleClientset(),
		mutex:                &sync.RWMutex{},
		nodeIps:              []string{"12.1.1.1"},
		podIPMap:             make(map[string]*PodInfo),
		serviceIPMap:         make(map[string]*ServiceInfo),
		labelTypeResourceMap: make(map[string]ResourcesOnLabel),
	}

	var pod corev1.Pod
	pod.Name = "test-pod"
	pod.Namespace = "test-ns"
	pod.Labels = map[string]string{"a": "b", "c": "d"}
	pod.Status.PodIP = "10.1.1.1"
	pod.Status.HostIP = "12.1.1.1"

	k8sManager.PodAdded(NewPodInfo(&pod))
	podInfo := k8sManager.GetPodFromIp("10.1.1.1")

	assert.NotNil(t, podInfo)
	assert.Equal(t, podInfo.PodIP, "10.1.1.1")
	assert.Equal(t, podInfo.Labels["a"], "b")
	assert.Equal(t, podInfo.Labels["c"], "d")
	assert.Equal(t, podInfo.Name(), "test-pod")
	assert.Equal(t, podInfo.Namespace(), "test-ns")
	assert.Equal(t, k8sManager.GetPodIpInThisNode(), "10.1.1.1")

	var service corev1.Service
	service.Name = "test-service"
	service.Namespace = "test-ns"
	service.Spec.Selector = map[string]string{"c": "d"}
	service.Spec.ClusterIP = "11.1.1.1"
	service.Spec.Ports = []corev1.ServicePort{{Name: "http", Port: 123, TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 456}}}

	k8sManager.ServiceAdded(NewServiceInfo(&service))

	serviceInfo := k8sManager.GetServiceFromClusterIp("11.1.1.1")
	assert.NotNil(t, serviceInfo)
	assert.Equal(t, serviceInfo.ClusterIP, "11.1.1.1")
	assert.Equal(t, serviceInfo.selector["c"], "d")
	assert.Equal(t, serviceInfo.Name(), "test-service")
	assert.Equal(t, serviceInfo.Namespace(), "test-ns")
	assert.Equal(t, serviceInfo.Ports[0].Port, uint32(123))
	assert.Equal(t, serviceInfo.Ports[0].TargetPort, uint32(456))
	assert.Equal(t, serviceInfo.Ports[0].Name, "http")

	pods := k8sManager.GetPodsForService(serviceInfo)
	assert.Equal(t, len(pods), 1)
	assert.Equal(t, pods[0], podInfo)

	var deploy v1beta1.Deployment
	deploy.Name = "test-deploy"
	deploy.Namespace = "test-ns"
	deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}}

	k8sManager.DeploymentAdded(NewDeploymentInfo(&deploy))
	deployment := k8sManager.GetPodDeployment(podInfo)

	assert.NotNil(t, deployment)
	assert.Equal(t, deployment.Name(), "test-deploy")
	assert.Equal(t, deployment.Namespace(), "test-ns")

	k8sManager.DeploymentDeleted(NewDeploymentInfo(&deploy))

	deployment = k8sManager.GetPodDeployment(podInfo)
	assert.Nil(t, deployment)

	k8sManager.PodDeleted(NewPodInfo(&pod))
	podInfo = k8sManager.GetPodFromIp("10.1.1.1")
	assert.Nil(t, podInfo)

	pods = k8sManager.GetPodsForService(serviceInfo)
	assert.Equal(t, len(pods), 0)

	k8sManager.ServiceDeleted(NewServiceInfo(&service))

	serviceInfo = k8sManager.GetServiceFromClusterIp("11.1.1.1")
	assert.Nil(t, serviceInfo)

}
