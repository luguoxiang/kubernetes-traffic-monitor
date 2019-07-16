# Introduction
Traffic-Monitor is a network sniffer program deployed on each node to collect traffic statistic information bewteen k8s pods. Currently only http traffic can be captured.

The captured traffic statistic information will be stored in build-in traffic-prometheus service.

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
# Show traffic by vizceral
```
kubectl apply -f deploy/vizceral.yaml
VIZCERAL_PORT=$(kubectl get svc traffic-vizceral -o=jsonpath="{.spec.ports[0].nodePort}")
browse http://localhost:$VIZCERAL_PORT/static/index.html
```
