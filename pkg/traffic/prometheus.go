package traffic

import (
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"os"
)

const (
	PROMETHEUS_DURATION_NAME = "request_duration_seconds"
	PROMETHEUS_COUNT_NAME    = "requests_total"
	SOURCE                   = "source"
	DESTINATION              = "destination"
	SOURCE_NAMESPACE         = "source_ns"
	DESTINATION_NAMESPACE    = "destination_ns"
	HTTP_METHOD              = "method"
	HTTP_URL                 = "url"
)

var (
	requestHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    PROMETHEUS_DURATION_NAME,
		Help:    "A histogram of the API HTTP request durations in seconds.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{SOURCE, SOURCE_NAMESPACE, DESTINATION, DESTINATION_NAMESPACE, HTTP_METHOD, HTTP_URL})

	requestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: PROMETHEUS_COUNT_NAME,
		Help: "API HTTP request count.",
	}, []string{SOURCE, SOURCE_NAMESPACE, DESTINATION, DESTINATION_NAMESPACE, HTTP_METHOD, HTTP_URL})
)

func init() {
	prometheus.MustRegister(requestHistogram)
	prometheus.MustRegister(requestCount)
	go func() {
		address := ":" + os.Getenv("VIZ_METRICS_PORT")
		glog.Infof("Running prometheus server on %s", address)
		glog.Infof("metrics: %s, %s", PROMETHEUS_COUNT_NAME, PROMETHEUS_DURATION_NAME)
		http.Handle("/metrics", promhttp.Handler())
		glog.Fatal(http.ListenAndServe(address, nil))
	}()
}

func SavePacket(info *TrafficInfo) {
	labels := prometheus.Labels{
		SOURCE:                info.Src,
		SOURCE_NAMESPACE:      info.SrcNS,
		DESTINATION:           info.Dst,
		DESTINATION_NAMESPACE: info.DstNS,
		HTTP_METHOD:           info.Method,
		HTTP_URL:              info.Url,
	}
	requestCount.With(labels).Inc()
	requestHistogram.With(labels).Observe(info.GetDurationTimeMiliSeconds() / 1000)
}
