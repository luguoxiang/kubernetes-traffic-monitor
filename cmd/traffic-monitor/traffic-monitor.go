package main

import (
	"flag"
	"github.com/luguoxiang/kubernetes-traffic-monitor/pkg/kubernetes"
	"github.com/luguoxiang/kubernetes-traffic-monitor/pkg/traffic"
)

func main() {
	flag.Parse()

	k8sManager, err := kubernetes.NewK8sResourceManager()
	if err != nil {
		panic(err.Error())
	}
	stopper := make(chan struct{})

	go k8sManager.WatchPods(stopper, k8sManager)
	go k8sManager.WatchDeployments(stopper, k8sManager)
	go k8sManager.WatchServices(stopper, k8sManager)
	go k8sManager.WatchStatefulSets(stopper, k8sManager)
	go k8sManager.WatchDaemonSets(stopper, k8sManager)

	packetManager, err := traffic.NewPacketManager(k8sManager)
	if err != nil {
		panic(err.Error())
	}
	packetManager.Run()
}
