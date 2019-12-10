package traffic

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/golang/glog"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"io"
	"net"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

type PCapManager struct {
	dockerNetIP   net.IP
	dockerNetMask net.IPMask
	pcapFilter    string
}

func (manager *PCapManager) InsideLocalPodIPRange(dstIp string) bool {
	netIp := net.ParseIP(dstIp)
	return manager.dockerNetIP.Equal(netIp.Mask(manager.dockerNetMask))
}

type PacketInfo struct {
	SrcPort       uint32
	DstPort       uint32
	SrcIp         string
	DstIp         string
	TimestampNano int64
	TcpTimestamp  []byte
	packet        gopacket.Packet
}

func (packet *PacketInfo) GetApplicationPayload() string {
	applicationLayer := packet.packet.ApplicationLayer()
	if applicationLayer != nil {
		return string(applicationLayer.Payload())
	}
	return ""
}
func (info *PacketInfo) String() string {
	var buffer bytes.Buffer
	buffer.WriteString(info.SrcIp)
	buffer.WriteString(":")
	buffer.WriteString(strconv.FormatInt(int64(info.SrcPort), 10))
	buffer.WriteString("=>")
	buffer.WriteString(info.DstIp)
	buffer.WriteString(":")
	buffer.WriteString(strconv.FormatInt(int64(info.DstPort), 10))
	return buffer.String()
}
func NewPacket(packet gopacket.Packet) *PacketInfo {
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if ipLayer == nil || tcpLayer == nil {
		glog.Warning("Unexpected packet, only IPv4 and TCP packet can be handled")
		for _, layer := range packet.Layers() {
			glog.Warning("PACKET LAYER:", layer.LayerType())
		}
		return nil
	}

	result := new(PacketInfo)
	result.packet = packet
	result.TimestampNano = packet.Metadata().Timestamp.UnixNano()
	tcp, _ := tcpLayer.(*layers.TCP)
	if len(tcp.Options) > 2 {
		result.TcpTimestamp = tcp.Options[2].OptionData
	}
	netInfo := packet.NetworkLayer().NetworkFlow()
	tcpInfo := packet.TransportLayer().TransportFlow()
	srcPort, err := strconv.ParseInt(tcpInfo.Src().String(), 10, 32)
	if err != nil {
		glog.Warningf("Unexpected source port %s", tcpInfo.Src().String())
		return nil
	}
	result.SrcPort = uint32(srcPort)

	dstPort, err := strconv.ParseInt(tcpInfo.Dst().String(), 10, 32)
	if err != nil {
		glog.Warningf("Unexpected destination port %s", tcpInfo.Dst().String())
		return nil
	}
	result.DstPort = uint32(dstPort)

	result.SrcIp = netInfo.Src().String()
	result.DstIp = netInfo.Dst().String()
	return result
}

func parseIp(ipHex string) (uint64, uint64, uint64, uint64, bool) {
	if len(ipHex) != 8 {
		return 0, 0, 0, 0, false
	}
	number0, err0 := strconv.ParseUint(ipHex[0:2], 16, 8)
	number1, err1 := strconv.ParseUint(ipHex[2:4], 16, 8)
	number2, err2 := strconv.ParseUint(ipHex[4:6], 16, 8)
	number3, err3 := strconv.ParseUint(ipHex[6:8], 16, 8)
	if err0 != nil || err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, 0, 0, false
	}
	return number0, number1, number2, number3, true
}

func getDefaultDevice(aPodIp net.IP) string {
	device := "docker0"
	fi, err := os.Open("/proc/net/route")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return device
	}
	defer fi.Close()

	blank := regexp.MustCompile("\\s+")

	first := true
	br := bufio.NewReader(fi)
	glog.Info("IP Route Table:")

	var maxMask uint64
	for {
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		line := string(a)
		items := blank.Split(line, -1)
		if first {
			first = false
			glog.Infof("%s %s %s", items[0], items[1], items[7])
			continue
		}
		ipHex := items[1]
		number0, number1, number2, number3, ok := parseIp(ipHex)
		if !ok {
			glog.Warningf("Unexpected Destination %s", ipHex)
			continue
		}
		ip := net.IPv4(byte(number3), byte(number2), byte(number1), byte(number0))

		ipHexMask := items[7]
		number0, number1, number2, number3, ok = parseIp(ipHexMask)
		if !ok {
			glog.Warningf("Unexpected Mask %s", ipHexMask)
			continue
		}
		ipMask := net.IPv4Mask(byte(number3), byte(number2), byte(number1), byte(number0))

		if number3 ==255 && number2==255 && number1==255 && number0 ==255 {
			glog.Infof("ignore %s %s", items[0], ip.String())
			continue
		}
		if ip.Equal(aPodIp.Mask(ipMask)) && maxMask < (number3+number2+number1+number0) {
			maxMask = number3 + number2 + number0 + number1
			device = items[0]
		}
		glog.Infof("%s %s %s", items[0], ip.String(), ipMask.String())

	}
	return device
}

type PacketHandler func(packet *PacketInfo)

func NewPCapManager(k8sIp string, aPodIp net.IP) *PCapManager {
	device := getDefaultDevice(aPodIp)

	var filters []string
	//pcap is only able to match data size of either 1, 2 or 4 bytes
	HTTP_HEADS := []string{"GET ", "PUT ", "POST", "DELE" /*DELETE*/, "HEAD", "HTTP"}

	for _, head := range HTTP_HEADS {
		hex := fmt.Sprintf("%x", []byte(head))
		filter := fmt.Sprintf("tcp[((tcp[12:1] & 0xf0) >> 2):4]=0x%s", hex)
		filters = append(filters, filter)
	}

	pcapFilter := strings.Join(filters, " or ")
	var dockerNetIP net.IP
	var dockerNetMask net.IPMask

	if k8sIp != "" {
		pcapFilter = fmt.Sprintf("%s and not host %s", pcapFilter, k8sIp)
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		glog.Warning("failed to get interfaces")
	} else {
		for _, iface := range ifaces {
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
					if ip.To4() == nil {
						continue
					}
					if iface.Name == "flannel0" {
						//Per https://github.com/coreos/flannel/issues/434, packet's source ip in receiver node
						//may be rewrite to flannel0's ip if docker's ip-masq is true, we need to ignore these packages
						pcapFilter = fmt.Sprintf("%s and not host %s", pcapFilter, ip.String())
						continue
					}
					if iface.Name == device {
						dockerNetIP = v.IP.Mask(v.Mask)
						dockerNetMask = v.Mask
					}
				default:
					break
				}
			}
		}
	}

	glog.Infof("docker ip: %s, mask: %s", dockerNetIP.String(), dockerNetMask.String())
	return &PCapManager{
		pcapFilter:    pcapFilter,
		dockerNetIP:   dockerNetIP,
		dockerNetMask: dockerNetMask,
	}
}

func (manager *PCapManager) Run(handler PacketHandler) {

	handle, err := pcap.OpenLive("any", 1024, false, pcap.BlockForever)
	if err != nil {
		panic(err)
	}
	defer handle.Close()

	err = handle.SetBPFFilter(manager.pcapFilter)
	if err != nil {
		panic(err)
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		_ = <-sigc
		glog.Warning("SIGTERM|SIGINT received, prepare to terminate")
		handle.Close()
	}()

	glog.Infof("pcap.OpenLive device=any, filter = %s",manager.pcapFilter)

	packetCh := make(chan *PacketInfo, 1000)
	go func() {
		for {
			info := <-packetCh

			bufLen := len(packetCh)
			if bufLen > 900 {
				glog.Warningf("packet buffer is about to be full, len =%d", bufLen)
			}

			handler(info)
		}
	}()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		p := NewPacket(packet)
		if p != nil {
			packetCh <- p
		}
	}
}
