# Introduction
Traffic-Monitor is a tools to monitor kubernetes network traffic between pods(Only HTTP is supported)

# Deploy traffic monitor
```
kubectl apply -f deploy/traffic-monitor.yaml
kubectl apply -f deploy/prometheus.yaml
```

# Deploy sample application
```
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.0/samples/bookinfo/platform/kube/bookinfo.yaml
```

# Generate traffic
```
kubectl port-forward deployments/productpage-v1 9080 &
while true; do curl http://localhost:9080/productpage; sleep 1;done
```

# Get traffic from promethues
```
kubectl port-forward deployments/traffic-prometheus 9090 &
curl localhost:9090/api/v1/query?query=requests_total|jq
```

