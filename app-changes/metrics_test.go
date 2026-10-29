// Reference copy. In pprof-demo this lives at internal/metrics/metrics_test.go.
package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMetricsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := New()
	r := gin.New()
	r.Use(m.Middleware())
	m.Register(r)
	r.GET("/api/ok", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/boom", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })

	for _, p := range []string{"/api/ok", "/api/ok", "/api/boom", "/nope"} {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, p, nil))
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/metrics status = %d", w.Code)
	}
	body := w.Body.String()

	for _, want := range []string{
		`pprofdemo_http_requests_total{code="200",method="GET",route="/api/ok"} 2`,
		`pprofdemo_http_requests_total{code="500",method="GET",route="/api/boom"} 1`,
		`pprofdemo_http_requests_total{code="404",method="GET",route="unmatched"} 1`,
		`pprofdemo_http_request_duration_seconds_bucket{method="GET",route="/api/ok",le="+Inf"} 2`,
		`pprofdemo_http_requests_in_flight 0`,
		`go_goroutines`,
		`process_cpu_seconds_total`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics output missing %q", want)
		}
	}
	if strings.Contains(body, `route="/metrics"`) {
		t.Error("/metrics scrapes must not be counted as application traffic")
	}
}
