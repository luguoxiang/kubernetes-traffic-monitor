package traffic

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/luguoxiang/kubernetes-traffic-monitor/pkg/kubernetes"
	"net"
	"regexp"
	"time"
)

var (
	httpRequestRegexp  = regexp.MustCompile(`^(GET|POST|PUT|DELETE|HEAD)\s+(.*)\sHTTP/[\d.]+`)
	httpResponseRegexp = regexp.MustCompile(`^HTTP/[\d.]+\s+(\d+)`)
)

type PacketManager struct {
	k8sManager     *kubernetes.K8sResourceManager
	pCapManager    *PCapManager
	trafficManager TrafficManager
}

func NewPacketManager(device string, k8sManager *kubernetes.K8sResourceManager) (*PacketManager, error) {
	k8sIp := k8sManager.GetK8sIP()
	if k8sIp == "" {
		glog.Warning("failed to get ip of 'kubernetes'")
	}

	var ip string
	for {
		ip = k8sManager.GetPodIpInThisNode()
		if ip != "" {
			break
		}
		glog.Warning("Failed to get a pod ip in this node, try again 10s later")
		time.Sleep(10 * time.Second)
	}
	return &PacketManager{
		k8sManager:  k8sManager,
		pCapManager: NewPCapManager(device, k8sIp, net.ParseIP(ip)),
	}, nil

}

func (manager *PacketManager) Run() {
	manager.pCapManager.Run(manager.Handle)
}

func (manager *PacketManager) checkResponse(packet *PacketInfo, srcPod *kubernetes.PodInfo, dstPod *kubernetes.PodInfo) (*TrafficInfo, bool) {
	trafficManager := manager.trafficManager
	pcapManager := manager.pCapManager
	k8sManager := manager.k8sManager

	var trafficInfo *TrafficInfo
	var duplicate bool
	//check if there is a request from packet.Dst to packet.Src. If this is true, this packet is a response
	if dstPod == nil {
		trafficInfo, duplicate = trafficManager.GetRequest("", packet.DstPort, packet.SrcIp, packet.SrcPort, packet.TcpTimestamp)
	} else {
		trafficInfo, duplicate = trafficManager.GetRequest(packet.DstIp, packet.DstPort, packet.SrcIp, packet.SrcPort, packet.TcpTimestamp)
	}
	if duplicate {
		return nil, false
	}
	if trafficInfo != nil {
		if dstPod != nil && srcPod != nil && !pcapManager.InsideLocalPodIPRange(packet.DstIp) {
			//Both SrcIp and DstIp are Pod IP
			//A cross nodes Pod to Pod request&response will generate two pair of packages, one pair for each node
			//The sender node's response package's source ip will be rewrite to service ip by kube-proxy iptable DNAT rule
			//the receiver does not have this rewrite, so we reject recevier node's package pair to avoid duplicate package counting.
			//Using sender side package pair can also make response time include the time spend in network

			//For in-node Pod to Pod request&response(DstIp will not be InsideLocalPodIPRange for cross-node response in receiver side)
			//there will only be one response package, so should not ignore(Because the cluster ip need to be DNAT,
			//the request package go-through docker0 twice, therefore there will be two request packages. One of them will be timeout and ignored)
			if glog.V(2) {
				glog.Infof("Ignore cross node POD Response: %s", packet.String())
			}
			return nil, false
		}

		return trafficInfo, false
	}

	//https://networkengineering.stackexchange.com/questions/18461/very-simple-nat-question-how-does-a-packet-get-back-out
	//In sender node, when a request dst service ip is DNAT to pod ip,
	//the corresponding response's source ip will be changed to service ip
	//This happens before the package arrived at docker0

	serviceInfo := k8sManager.GetServiceFromClusterIp(packet.SrcIp)

	if serviceInfo == nil {
		return nil, true
	}

	var srcPortInfo *kubernetes.ServicePortInfo
	for _, port := range serviceInfo.Ports {
		if packet.SrcPort == port.Port {
			srcPortInfo = port
			break
		}
	}
	if srcPortInfo == nil {
		if glog.V(2) {
			glog.Infof("Found source service %s, but no port match %d", serviceInfo.Name(), packet.SrcPort)
		}
		return nil, true
	}
	for _, pod := range k8sManager.GetPodsForService(serviceInfo) {
		deployment := k8sManager.GetPodDeployment(pod)
		if deployment == nil {
			continue
		}
		//find the service's corresponding pod ip
		for _, port := range deployment.Ports {
			if port == srcPortInfo.TargetPort {
				var duplicate bool
				if dstPod == nil {
					trafficInfo, duplicate = trafficManager.GetRequest("", packet.DstPort, pod.PodIP, srcPortInfo.TargetPort, packet.TcpTimestamp)
				} else {
					trafficInfo, duplicate = trafficManager.GetRequest(packet.DstIp, packet.DstPort, pod.PodIP, srcPortInfo.TargetPort, packet.TcpTimestamp)
				}
				if duplicate {
					return nil, false
				}
				if trafficInfo != nil {
					if glog.V(2) {
						glog.Infof("Map Service IP %s to Pod IP %s", packet.SrcIp, pod.PodIP)
					}
					return trafficInfo, false
				}
				if glog.V(2) {
					if dstPod == nil {
						glog.Infof("Could not found request from INTERNET:%d to %s:%d ", packet.DstPort, pod.PodIP, srcPortInfo.TargetPort)
					} else {
						glog.Infof("Could not found request from %s:%d to %s:%d ", packet.DstIp, packet.DstPort, pod.PodIP, srcPortInfo.TargetPort)
					}
				}
			}
		}
	}
	if glog.V(2) {
		glog.Infof("Found source service %s:%d, but could not found request target at it", serviceInfo.Name(), srcPortInfo.TargetPort)
	}
	return nil, true

}
func (manager *PacketManager) Handle(packet *PacketInfo) {
	k8sManager := manager.k8sManager
	trafficManager := &manager.trafficManager

	//https://superuser.com/questions/925286/does-tcpdump-bypass-iptables
	//For request, gopacket capture packages after iptable's process, so the DstIp has been DNAT to PodIP

	srcPod := k8sManager.GetPodFromIp(packet.SrcIp)

	if srcPod != nil && srcPod.IsSkip() {
		return
	}

	dstPod := k8sManager.GetPodFromIp(packet.DstIp)
	if dstPod != nil && dstPod.IsSkip() {
		return
	}

	trafficInfo, mayBeRequest := manager.checkResponse(packet, srcPod, dstPod)
	if trafficInfo != nil {
		content := packet.GetApplicationPayload()
		match := httpResponseRegexp.FindStringSubmatch(content)
		if match != nil && len(match) > 1 {
			trafficInfo.SetResponse(match[1], packet.TimestampNano, packet.TcpTimestamp)
			if glog.V(2) {
				glog.Infof("RESPONSE %s %d", trafficInfo.String(), len(content))
			}
			SavePacket(trafficInfo)
		} else {
			if glog.V(2) {
				glog.Infof("RESPONSE %s CONTINUE %d", trafficInfo.String(), len(content))
			}
		}
		return
	} else if !mayBeRequest {
		return
	}

	dstDeployment := k8sManager.GetPodDeployment(dstPod)
	if dstDeployment == nil {
		if glog.V(2) {
			glog.Info(fmt.Sprintf("SKIP FOR UNKNOWN DST %s:%d", packet.DstIp, packet.DstPort))
		}
		return
	}
	srcDeployment := k8sManager.GetPodDeployment(srcPod)
	for _, port := range dstDeployment.Ports {
		if port == packet.DstPort {
			content := packet.GetApplicationPayload()
			match := httpRequestRegexp.FindStringSubmatch(content)
			if match != nil && len(match) > 2 {
				trafficInfo := NewTrafficInfo(packet, match[2], match[1])
				trafficInfo.Dst = dstDeployment.Name()
				trafficInfo.DstNS = dstPod.Namespace()
				if srcPod != nil && srcDeployment != nil {
					trafficInfo.Src = srcDeployment.Name()
					trafficInfo.SrcNS = srcPod.Namespace()
				}
				trafficManager.AddRequest(trafficInfo)
				return
			}
		}
	}

	if glog.V(2) {
		glog.Info(fmt.Sprintf("UNKNOWN %s", packet.String()))
	}
}
