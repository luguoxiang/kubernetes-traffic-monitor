VERSION=1.0

vendor:
	dep ensure -vendor-only -v

clean:
	rm -f traffic-monitor

build: vendor
	go build -o traffic-monitor cmd/traffic-monitor/traffic-monitor.go

test: vendor
	go test -v github.com/luguoxiang/kubernetes-traffic-monitor/pkg/...

build.images.vizceral: 
	(cd vizceral;docker build -t luguoxiang/traffic-vizceral:${VERSION} .;docker push luguoxiang/traffic-vizceral:${VERSION})

build.images: 
	docker build -t luguoxiang/traffic-monitor:${VERSION} .
	docker push luguoxiang/traffic-monitor:${VERSION}

