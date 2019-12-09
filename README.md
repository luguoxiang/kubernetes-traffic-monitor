# Introduction
Traffic-Monitor is a network sniffer program deployed on each node to collect traffic statistic information bewteen k8s pods. Currently only http traffic can be captured.

The captured traffic statistic information will be stored in build-in traffic-prometheus service.

# Deploy traffic monitor
```
git clone https://github.com/luguoxiang/kubernetes-traffic-manager.git
helm install --set monitor.enabled=true --name kubernetes-traffic-manager helm/kubernetes-traffic-manager
```

# Deploy sample application
```
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.0/samples/bookinfo/platform/kube/bookinfo.yaml
```

# Generate traffic
```
kubectl apply -f deploy/vizceral.yaml
kubectl apply -f vizceral/ingress.yaml
INGRESS_HOST=`kubectl get svc traffic-ingress -o jsonpath='{.status.loadBalancer.ingress[0].hostname}'`
while true; do curl http://${INGRESS_HOST}/productpage; sleep 1;done
```

# Get traffic from promethues
```
curl ${INGRESS_HOST}/api/v1/query?query=requests_total|jq
```
# Show traffic by vizceral
```
browse http://${INGRESS_HOST}/static/index.html
```
