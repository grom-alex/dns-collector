package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// GinMiddleware returns a Gin middleware that records HTTP metrics.
func GinMiddleware(registry *Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip metrics endpoint itself
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		method := c.Request.Method

		// Track requests in flight
		registry.HTTPRequestsInFlight.Inc()
		defer registry.HTTPRequestsInFlight.Dec()

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		responseSize := float64(c.Writer.Size())

		registry.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		registry.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
		if responseSize >= 0 {
			registry.HTTPResponseSize.WithLabelValues(method, path).Observe(responseSize)
		}

		// Track specific API calls
		switch path {
		case "/api/stats":
			registry.APIStatsQueries.Inc()
		case "/api/domains":
			registry.APIDomainsQueries.Inc()
		case "/api/stats/export":
			registry.APIExportTotal.WithLabelValues("stats_excel").Inc()
		case "/api/domains/export":
			registry.APIExportTotal.WithLabelValues("domains_excel").Inc()
		}
	}
}

// PrometheusHandler returns a Gin handler for the /metrics endpoint.
func PrometheusHandler(registry *Registry) gin.HandlerFunc {
	h := promhttp.HandlerFor(
		registry.GetRegistry(),
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		},
	)
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
