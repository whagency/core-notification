package tools

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	ginprometheus "github.com/willthrom/go-gin-prometheus"
)

const _serviceName = "notification_service_"

var defaultBuckets = []float64{0.05, 0.15, 0.3, 0.6, 1, 2, 5}

var (
	dbRequestError = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: _serviceName + "db_error_count",
			Help: "Number of DB Errors",
		},
	)

	dbRequestTime = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:        _serviceName + "db_request_time",
			Help:        "DB requests time in seconds",
			ConstLabels: map[string]string{},
			Buckets:     defaultBuckets,
		},
	)

	cacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: _serviceName + "cache_hits",
			Help: "Number of cache hits",
		},
		[]string{"entity"},
	)

	cacheMissing = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: _serviceName + "cache_missing",
			Help: "Number of cache missed",
		},
		[]string{"entity"},
	)
)

// RegisterMetrics registers all the metrics with Prometheus
func RegisterMetrics() {
	prometheus.MustRegister(
		cacheHits,
		cacheMissing,
		dbRequestError,
		dbRequestTime,
	)
}

// LinkToPrometheusToGin Link Prometheus to Gin
func LinkToPrometheusToGin(ge *gin.Engine) {
	p := ginprometheus.NewPrometheusZap(_serviceName+"gin", nil)
	p.ReqCntURLLabelMappingFn = func(c *gin.Context) string {
		url := c.Request.URL.Path
		for _, param := range c.Params {
			url = strings.Replace(url, param.Value, ":"+param.Key, 1)
		}
		return url
	}
	p.Use(ge)
}

// TrackMetricDBErrorAdd Increment DB error counter
func TrackMetricDBErrorAdd() {
	dbRequestError.Inc()
}

// EndDBRequestTimer Record timing for DB request
func EndDBRequestTimer(startTime time.Time) {
	dbRequestTime.Observe(time.Since(startTime).Seconds())
}

// TrackMetricCacheHitsAdd Increment Local Cache counter
func TrackMetricCacheHitsAdd(entity string) {
	cacheHits.WithLabelValues(entity).Inc()
}

// TrackMetricCacheMissedAdd Increment Missed Local Cache counter
func TrackMetricCacheMissedAdd(entity string) {
	cacheMissing.WithLabelValues(entity).Inc()
}
