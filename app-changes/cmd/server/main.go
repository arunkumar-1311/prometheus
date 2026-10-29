// Standalone server: CRUD API (/api/items) + Prometheus metrics (/metrics).
//
//	cd prometheus/app-changes
//	go run ./cmd/server            # listens on :8080 (override with APP_ADDR=:9000)
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"example.com/metrics-reference"
	"example.com/metrics-reference/items"
)

func main() {
	addr := os.Getenv("APP_ADDR")
	if addr == "" {
		addr = ":8080" // matches the scrape target in docker/prometheus.yml and local/prometheus.yml
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	
	m := metrics.New()
	r.Use(m.Middleware())
	m.Register(r) // GET /metrics
	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	items.NewHandler(items.NewStore(), m.Registerer()).Register(r) // /api/items

	srv := &http.Server{Addr: addr, Handler: r, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("listening on %s (metrics: /metrics, api: /api/items)", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
