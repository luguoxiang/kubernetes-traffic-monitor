package kubernetes

import (
	"fmt"
	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"net"
	"os"
	"sync"
	"sync/atomic"
)

type ResourcesOnLabel map[ResourceType][]ResourceInfoPointer

type K8sResourceManager struct {
	podIPMap             map[string]*PodInfo
	serviceIPMap         map[string]*ServiceInfo
	labelTypeResourceMap map[string]ResourcesOnLabel
	clientSet            kubernetes.Interface
	mutex                *sync.RWMutex
	locked               int32

	nodeIps         []string
	podIpInThisNode string
}

func NewK8sResourceManager() (*K8sResourceManager, error) {

	clientSet, err := getK8sClientSet()
	if err != nil {
		return nil, err
	}

	result := &K8sResourceManager{
		clientSet: clientSet,

		mutex: &sync.RWMutex{},
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("Failed to get Interfaces: %s", err.Error())
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return nil, fmt.Errorf("Failed to get interface address: %s", err.Error())
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip.To4() == nil {
				continue
			}
			result.nodeIps = append(result.nodeIps, ip.String())
		}
	}

	result.podIPMap = make(map[string]*PodInfo)
	result.serviceIPMap = make(map[string]*ServiceInfo)
	result.labelTypeResourceMap = make(map[string]ResourcesOnLabel)
	return result, nil
}
func (manager *K8sResourceManager) NewCond() *sync.Cond {
	return sync.NewCond(manager.mutex)
}
func (manager *K8sResourceManager) Lock() {
	manager.mutex.Lock()
	atomic.AddInt32(&manager.locked, 1)
}
func (manager *K8sResourceManager) Unlock() {
	atomic.StoreInt32(&manager.locked, 0)
	manager.mutex.Unlock()
}

func (manager *K8sResourceManager) IsLocked() bool {
	return atomic.LoadInt32(&manager.locked) != 0
}

func (manager *K8sResourceManager) GetPodsForService(service *ServiceInfo) []*PodInfo {
	manager.Lock()
	defer manager.Unlock()

	pods := manager.GetMatchedResources(service, POD_TYPE)

	var result []*PodInfo
	for _, pod := range pods {
		result = append(result, pod.(*PodInfo))
	}
	return result
}

func (manager *K8sResourceManager) GetPodDeployment(pod *PodInfo) *DeploymentInfo {
	if pod == nil {
		return nil
	}
	manager.Lock()
	defer manager.Unlock()

	resources := manager.GetMatchedResources(pod, DEPLOYMENT_TYPE)
	var result *DeploymentInfo
	for _, resource := range resources {
		if result == nil || len(result.GetSelector()) < len(resource.GetSelector()) {
			//return matched deployment which has max number of selectors
			result = resource.(*DeploymentInfo)
		}
	}
	return result
}

func (manager *K8sResourceManager) GetPodFromIp(ip string) *PodInfo {
	manager.Lock()
	defer manager.Unlock()

	return manager.podIPMap[ip]
}

func (manager *K8sResourceManager) GetPodIpInThisNode() string {
	return manager.podIpInThisNode
}
func (manager *K8sResourceManager) GetServiceFromClusterIp(ip string) *ServiceInfo {
	manager.Lock()
	defer manager.Unlock()

	return manager.serviceIPMap[ip]
}

func getK8sClientSet() (kubernetes.Interface, error) {
	configPath := os.Getenv("KUBECONFIG")

	var config *rest.Config
	var err error
	if configPath == "" {
		glog.Info("KUBECONFIG: InCluster\n")
		config, err = rest.InClusterConfig()
	} else {
		glog.Infof("KUBECONFIG:%s\n", configPath)
		config, err = clientcmd.BuildConfigFromFlags("", configPath)
	}
	if err != nil {
		return nil, err
	}

	// create the clientset
	return kubernetes.NewForConfig(config)
}

func (manager *K8sResourceManager) GetK8sIP() string {
	service, err := manager.clientSet.CoreV1().Services("default").Get("kubernetes", metav1.GetOptions{})
	if err != nil {
		return ""
	}
	return service.Spec.ClusterIP
}
