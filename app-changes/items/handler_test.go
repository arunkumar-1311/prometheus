package items

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

func newRouter(t *testing.T) (*gin.Engine, *prometheus.Registry) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	reg := prometheus.NewRegistry()
	r := gin.New()
	NewHandler(NewStore(), reg).Register(r)
	return r, reg
}

func do(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// metric returns the value of a counter/gauge series from reg; labels may be nil.
func metric(t *testing.T, reg *prometheus.Registry, name string, labels map[string]string) float64 {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
	next:
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				if want, ok := labels[lp.GetName()]; ok && want != lp.GetValue() {
					continue next
				}
			}
			if m.GetCounter() != nil {
				return m.GetCounter().GetValue()
			}
			return m.GetGauge().GetValue()
		}
	}
	t.Fatalf("metric %s%v not found", name, labels)
	return 0
}

func TestCRUDFlowAndMetrics(t *testing.T) {
	r, reg := newRouter(t)

	// Create
	w := do(r, http.MethodPost, "/api/items", `{"name":"pen","price":1.5,"quantity":10}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", w.Code, w.Body)
	}
	var created Item
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created.ID != 1 || created.Name != "pen" {
		t.Fatalf("created = %+v", created)
	}

	// Read
	if w = do(r, http.MethodGet, "/api/items/1", ""); w.Code != http.StatusOK {
		t.Fatalf("get = %d", w.Code)
	}
	if w = do(r, http.MethodGet, "/api/items", ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"count":1`) {
		t.Fatalf("list = %d %s", w.Code, w.Body)
	}

	// Update
	w = do(r, http.MethodPut, "/api/items/1", `{"name":"pencil","price":2,"quantity":5}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"pencil"`) {
		t.Fatalf("update = %d %s", w.Code, w.Body)
	}

	// Errors: unknown id, bad id, bad body
	if w = do(r, http.MethodGet, "/api/items/99", ""); w.Code != http.StatusNotFound {
		t.Fatalf("get missing = %d", w.Code)
	}
	if w = do(r, http.MethodGet, "/api/items/abc", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("get bad id = %d", w.Code)
	}
	if w = do(r, http.MethodPost, "/api/items", `{"price":-1}`); w.Code != http.StatusBadRequest {
		t.Fatalf("create invalid = %d", w.Code)
	}

	if got := metric(t, reg, "pprofdemo_items_stored", nil); got != 1 {
		t.Errorf("items_stored before delete = %v, want 1", got)
	}

	// Delete
	if w = do(r, http.MethodDelete, "/api/items/1", ""); w.Code != http.StatusNoContent {
		t.Fatalf("delete = %d", w.Code)
	}
	if w = do(r, http.MethodDelete, "/api/items/1", ""); w.Code != http.StatusNotFound {
		t.Fatalf("delete again = %d", w.Code)
	}

	for _, tc := range []struct {
		op, result string
		want       float64
	}{
		{"create", "success", 1}, {"create", "invalid", 1},
		{"get", "success", 1}, {"get", "not_found", 1}, {"get", "invalid", 1},
		{"list", "success", 1}, {"update", "success", 1},
		{"delete", "success", 1}, {"delete", "not_found", 1},
	} {
		got := metric(t, reg, "pprofdemo_item_operations_total", map[string]string{"operation": tc.op, "result": tc.result})
		if got != tc.want {
			t.Errorf("operations{%s,%s} = %v, want %v", tc.op, tc.result, got, tc.want)
		}
	}
	if got := metric(t, reg, "pprofdemo_items_stored", nil); got != 0 {
		t.Errorf("items_stored after delete = %v, want 0", got)
	}
}
