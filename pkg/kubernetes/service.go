package kubernetes

import (
	"bytes"
	"fmt"
	"k8s.io/api/core/v1"
)

type ServicePortInfo struct {
	Port       uint32
	TargetPort uint32
	Name       string
}
type ServiceInfo struct {
	ResourceVersion string
	name            string
	namespace       string
	ClusterIP       string
	selector        map[string]string
	Ports           []*ServicePortInfo
}

func (service *ServiceInfo) Type() ResourceType {
	return SERVICE_TYPE
}

func (service *ServiceInfo) IsKubeAPIService() bool {
	return service.Name() == "kubernetes" && service.Namespace() == "default"
}

func (service *ServiceInfo) Name() string {
	return service.name
}

func (service *ServiceInfo) Namespace() string {
	return service.namespace
}

func (service *ServiceInfo) GetSelector() map[string]string {
	return service.selector
}

func (service *ServiceInfo) String() string {
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("Service %s@%s", service.name, service.namespace))
	for _, port := range service.Ports {
		buffer.WriteString(fmt.Sprintf("%d", port.Port))
		buffer.WriteString(" ")
	}

	return buffer.String()
}

func NewServiceInfo(service *v1.Service) *ServiceInfo {

	info := &ServiceInfo{
		name:            service.Name,
		namespace:       service.Namespace,
		selector:        service.Spec.Selector,
		ClusterIP:       service.Spec.ClusterIP,
		ResourceVersion: service.ResourceVersion,
	}
	for _, port := range service.Spec.Ports {
		var targetPort uint32
		if port.TargetPort.IntVal > 0 {
			targetPort = uint32(port.TargetPort.IntVal)
		}

		info.Ports = append(info.Ports, &ServicePortInfo{
			Name:       port.Name,
			Port:       uint32(port.Port),
			TargetPort: targetPort,
		})
	}

	return info

}
