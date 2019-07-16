vendor:
	dep ensure -vendor-only -v

clean:
	rm -f traffic-monitor

build: vendor
	go build -o traffic-monitor cmd/traffic-monitor/traffic-monitor.go

build.images: 
	(cd vizceral;make build.images)
	docker build -t luguoxiang/traffic-monitor .

push.images:
	(cd vizceral;make push.images)
	docker push luguoxiang/traffic-monitor
