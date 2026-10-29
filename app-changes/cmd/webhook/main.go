// Tiny Alertmanager webhook receiver that prints every alert it gets.
// Use it to see the routing work without Slack or email.
//
//	cd prometheus/app-changes
//	go run ./cmd/webhook           # listens on :5001 (override with WEBHOOK_ADDR)
//
// Alertmanager posts to /default, /critical and /warning (see alertmanager.yml).
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

// payload is the subset of the Alertmanager webhook JSON this demo prints.
type payload struct {
	Status   string `json:"status"`
	Receiver string `json:"receiver"`
	Alerts   []struct {
		Status      string            `json:"status"`
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
	} `json:"alerts"`
}

func main() {
	addr := os.Getenv("WEBHOOK_ADDR")
	if addr == "" {
		addr = ":5001"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var p payload
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&p); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		log.Printf("[%s] route=%s receiver=%s alerts=%d", p.Status, r.URL.Path, p.Receiver, len(p.Alerts))
		for _, a := range p.Alerts {
			log.Printf("    %-8s %-15s severity=%-8s %s",
				a.Status, a.Labels["alertname"], a.Labels["severity"], a.Annotations["summary"])
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("webhook receiver listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
}
