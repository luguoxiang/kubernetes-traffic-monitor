package traffic

import (
	"bytes"
	"github.com/golang/glog"
	"strconv"
)

type TrafficInfo struct {
	SrcPort               uint32
	DstPort               uint32
	SrcIP                 string
	DstIP                 string
	Src                   string
	Dst                   string
	SrcNS                 string
	DstNS                 string
	Url                   string
	Method                string
	Status                string
	TcpRequestTimestamp   []byte
	TcpResponseTimestamp  []byte
	requestTimestampNano  int64
	responseTimestampNano int64
	Next                  *TrafficInfo
}

func (info *TrafficInfo) GetDurationTimeMiliSeconds() float64 {
	a := info.responseTimestampNano / 1000
	b := info.requestTimestampNano / 1000
	if a <= b {
		return 0
	}
	return float64(a-b) / 1000
}

func (info *TrafficInfo) getRequestTimestampMiliSeconds() int64 {
	return info.requestTimestampNano / 1e6
}

func (info *TrafficInfo) String() string {
	var buffer bytes.Buffer
	if info.Src == "" {
		buffer.WriteString("INTERNET(")
		buffer.WriteString(info.SrcIP)
		buffer.WriteString(")")
	} else {
		buffer.WriteString(info.Src)
	}

	buffer.WriteString(":")
	buffer.WriteString(strconv.FormatInt(int64(info.SrcPort), 10))
	buffer.WriteString("=>")
	buffer.WriteString(info.Dst)
	buffer.WriteString(":")
	buffer.WriteString(strconv.FormatInt(int64(info.DstPort), 10))
	buffer.WriteString(" ")
	buffer.WriteString(info.Method)
	buffer.WriteString(" ")
	buffer.WriteString(info.Url)
	return buffer.String()
}

func (info *TrafficInfo) SetResponse(status string, responseTimestampNano int64, tcpResponseTimestamp []byte) {
	info.Status = status
	info.TcpResponseTimestamp = tcpResponseTimestamp
	info.responseTimestampNano = responseTimestampNano
}

func NewTrafficInfo(packet *PacketInfo, url string, method string) *TrafficInfo {
	return &TrafficInfo{
		SrcIP:                packet.SrcIp,
		DstIP:                packet.DstIp,
		SrcPort:              packet.SrcPort,
		DstPort:              packet.DstPort,
		Url:                  url,
		Method:               method,
		requestTimestampNano: packet.TimestampNano,
		TcpRequestTimestamp:  packet.TcpTimestamp}
}

type packetNode struct {
	Traffic   *TrafficInfo
	Timestamp int64
	Next      *packetNode
}

const TIME_RANGE = 60 * 1000 //miliseconds

type TrafficManager struct {
	allPackets  [TIME_RANGE]*packetNode
	allRequests [65536]*TrafficInfo
}

func (manager *TrafficManager) GetRequest(srcIp string, srcPort uint32, dstIp string, dstPort uint32, tcpResponseTimestamp []byte) (*TrafficInfo, bool /*duplicate*/) {
	request := manager.allRequests[srcPort]
	var firstMatch *TrafficInfo
	for request != nil {
		if request.DstPort == dstPort && request.DstIP == dstIp {
			if request.TcpResponseTimestamp != nil {
				if bytes.Compare(request.TcpResponseTimestamp, tcpResponseTimestamp) == 0 {
					if glog.V(2) {
						glog.Info("duplicate response ", request.String())
					}
					return nil, true
				}
				if firstMatch == nil {
					firstMatch = request
				}
			} else if srcIp == "" && request.Src == "" {
				return request, false
			} else if srcIp == request.SrcIP {
				return request, false
			}
		}
		request = request.Next
	}

	return firstMatch, false
}

func (manager *TrafficManager) removeTraffic(info *TrafficInfo) {
	request := manager.allRequests[info.SrcPort]
	if request == nil {
		glog.Warning("Could not remove ", info.String())
		return
	}
	if request == info {
		manager.allRequests[info.SrcPort] = request.Next
	} else {
		for request.Next != nil && request.Next != info {
			request = request.Next
		}

		if request.Next != nil {
			request.Next = request.Next.Next
		} else {
			glog.Warning("Could not remove ", info.String())
			return
		}
	}
	if glog.V(2) {
		glog.Info("Removed ", info.String())
	}
}

func (manager *TrafficManager) addTraffic(info *TrafficInfo) bool {
	request := manager.allRequests[info.SrcPort]
	for request != nil {
		if request == info || bytes.Compare(request.TcpRequestTimestamp, info.TcpRequestTimestamp) == 0 {
			if glog.V(2) {
				glog.Info("duplicate request ", info.String())
			}
			return false
		}
		request = request.Next
	}
	info.Next = manager.allRequests[info.SrcPort]
	manager.allRequests[info.SrcPort] = info

	return true
}

func (manager *TrafficManager) addPacket(info *TrafficInfo) {
	ts := info.getRequestTimestampMiliSeconds() % TIME_RANGE
	packet := manager.allPackets[ts]

	timestamp := info.getRequestTimestampMiliSeconds()
	for packet != nil && packet.Timestamp+TIME_RANGE <= timestamp {
		manager.removeTraffic(packet.Traffic)
		packet = packet.Next
	}

	if packet == nil || packet.Timestamp > timestamp {
		manager.allPackets[ts] = &packetNode{Traffic: info, Timestamp: timestamp, Next: packet}
	} else {
		manager.allPackets[ts] = packet
		for packet.Next != nil && packet.Next.Timestamp <= timestamp {
			packet = packet.Next
		}
		packet.Next = &packetNode{Traffic: info, Timestamp: timestamp, Next: packet.Next}
	}
}

func (manager *TrafficManager) AddRequest(info *TrafficInfo) {
	if int(info.SrcPort) >= len(manager.allRequests) {
		glog.Errorf("unexpected source port number:%d", info.SrcPort)
		return
	}
	if manager.addTraffic(info) {
		manager.addPacket(info)
		if glog.V(2) {
			glog.Infof("REQUEST %s", info.String())
		}
	}
}
