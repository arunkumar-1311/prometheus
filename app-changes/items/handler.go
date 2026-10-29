package items

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// Operation and result label values for pprofdemo_item_operations_total.
const (
	opCreate = "create"
	opList   = "list"
	opGet    = "get"
	opUpdate = "update"
	opDelete = "delete"

	resSuccess  = "success"
	resInvalid  = "invalid"   // bad id or body (HTTP 400)
	resNotFound = "not_found" // HTTP 404
)

type Handler struct {
	store *Store
	ops   *prometheus.CounterVec
}

// NewHandler registers two metrics on reg:
//
//	pprofdemo_item_operations_total{operation,result}  counter
//	pprofdemo_items_stored                             gauge (current item count)
func NewHandler(store *Store, reg prometheus.Registerer) *Handler {
	ops := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "pprofdemo",
		Name:      "item_operations_total",
		Help:      "Item CRUD operations by operation and result.",
	}, []string{"operation", "result"})
	stored := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "pprofdemo",
		Name:      "items_stored",
		Help:      "Number of items currently stored.",
	}, func() float64 { return float64(store.Len()) })
	reg.MustRegister(ops, stored)

	// Create every series up front so graphs show 0 instead of "no data".
	for _, op := range []string{opCreate, opList, opGet, opUpdate, opDelete} {
		for _, res := range []string{resSuccess, resInvalid, resNotFound} {
			ops.WithLabelValues(op, res)
		}
	}
	return &Handler{store: store, ops: ops}
}

func (h *Handler) Register(r *gin.Engine) {
	g := r.Group("/api/items")
	g.POST("", h.create)
	g.GET("", h.list)
	g.GET("/:id", h.get)
	g.PUT("/:id", h.update)
	g.DELETE("/:id", h.remove)
}

func (h *Handler) record(op, result string) { h.ops.WithLabelValues(op, result).Inc() }

func (h *Handler) invalid(c *gin.Context, op string, err error) {
	h.record(op, resInvalid)
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func (h *Handler) notFound(c *gin.Context, op string) {
	h.record(op, resNotFound)
	c.JSON(http.StatusNotFound, gin.H{"error": ErrNotFound.Error()})
}

func parseID(c *gin.Context) (int, error) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		return 0, errors.New("id must be a positive integer")
	}
	return id, nil
}

// POST /api/items  {"name":"pen","price":1.5,"quantity":10}
func (h *Handler) create(c *gin.Context) {
	var in Input
	if err := c.ShouldBindJSON(&in); err != nil {
		h.invalid(c, opCreate, err)
		return
	}
	it := h.store.Create(in)
	h.record(opCreate, resSuccess)
	c.JSON(http.StatusCreated, it)
}

// GET /api/items
func (h *Handler) list(c *gin.Context) {
	list := h.store.List()
	h.record(opList, resSuccess)
	c.JSON(http.StatusOK, gin.H{"count": len(list), "items": list})
}

// GET /api/items/:id
func (h *Handler) get(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		h.invalid(c, opGet, err)
		return
	}
	it, err := h.store.Get(id)
	if err != nil {
		h.notFound(c, opGet)
		return
	}
	h.record(opGet, resSuccess)
	c.JSON(http.StatusOK, it)
}

// PUT /api/items/:id  (full replace of name, price, quantity)
func (h *Handler) update(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		h.invalid(c, opUpdate, err)
		return
	}
	var in Input
	if err := c.ShouldBindJSON(&in); err != nil {
		h.invalid(c, opUpdate, err)
		return
	}
	it, err := h.store.Update(id, in)
	if err != nil {
		h.notFound(c, opUpdate)
		return
	}
	h.record(opUpdate, resSuccess)
	c.JSON(http.StatusOK, it)
}

// DELETE /api/items/:id
func (h *Handler) remove(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		h.invalid(c, opDelete, err)
		return
	}
	if err := h.store.Delete(id); err != nil {
		h.notFound(c, opDelete)
		return
	}
	h.record(opDelete, resSuccess)
	c.Status(http.StatusNoContent)
}
