# Prometheus + Grafana for pprof-demo

Monitoring stack for the Go app in [../pprof-demo](../pprof-demo). The app exposes
`GET /metrics`, Prometheus scrapes it, Grafana draws the dashboard. pprof tells you
*where* a problem is; these graphs tell you *when* it started and how bad it is.

```
prometheus/
├── README.md
├── alerts.yml                           alert rules (shared by both setups)
├── grafana/dashboards/pprof-demo.json   dashboard, 11 panels (shared by both setups)
├── docker/                              Option A: Docker
│   ├── docker-compose.yml
│   ├── prometheus.yml                   scrapes host.docker.internal:8080, sends alerts to Alertmanager
│   ├── alertmanager.yml                 alert routing + receivers
│   └── grafana-provisioning/            data source + dashboard auto-load
├── local/                               Option B: no Docker
│   ├── prometheus.yml                   scrapes localhost:8080, sends alerts to localhost:9093
│   ├── alertmanager.yml                 alert routing + receivers
│   └── grafana-datasource.yml
└── app-changes/                         reference copies of the Go code
    ├── go.mod, go.sum                   own Go module (same versions as pprof-demo)
    ├── cmd/server/main.go               runnable server: CRUD + /metrics (go run ./cmd/server)
    ├── cmd/webhook/main.go              prints alerts sent by Alertmanager (go run ./cmd/webhook)
    ├── metrics.go                       the instrumentation: middleware + /metrics
    ├── metrics_test.go
    └── items/                           CRUD example with its own metrics
        ├── store.go                     in-memory create/read/update/delete
        ├── handler.go                   Gin routes + Prometheus counters/gauge
        └── handler_test.go
```

Commands are shown for **bash** (Git Bash, Linux, macOS) and **PowerShell**.
In PowerShell use `curl.exe`, not `curl` (that is an alias for `Invoke-WebRequest`).

| What | URL | Port |
|---|---|---|
| App + `/metrics` | http://localhost:8080 | 8080 |
| pprof (localhost only) | http://localhost:6060/debug/pprof/ | 6060 |
| Prometheus | http://localhost:9090 | 9090 |
| Alertmanager | http://localhost:9093 | 9093 |
| Webhook receiver (demo) | http://localhost:5001 | 5001 |
| Grafana | http://localhost:3000 (admin / admin) | 3000 |

---

## 1. App changes (needed for both options)

Already applied in `pprof-demo`. To repeat them in another Gin app:

1. Add the dependency:
   ```
   go get github.com/prometheus/client_golang@v1.20.5
   go mod tidy
   ```
2. Copy [app-changes/metrics.go](app-changes/metrics.go) to
   `internal/metrics/metrics.go` and [app-changes/metrics_test.go](app-changes/metrics_test.go)
   to `internal/metrics/metrics_test.go`. `app-changes` is its own Go module
   (`go test ./...` works inside it), so the IDE resolves the imports there too.
   Do not copy its `go.mod`; use your app's.
3. Edit `cmd/server/main.go`:
   ```go
   import "pprof-demo/internal/metrics"   // adjust to your module name

   r := gin.New()
   r.Use(gin.Recovery())
   m := metrics.New()
   r.Use(m.Middleware())   // must come before the routes you want measured
   m.Register(r)           // GET /metrics
   handlers.New().Register(r)
   ```
4. Test and run:
   ```
   cd pprof-demo
   go test ./internal/metrics/
   go run ./cmd/server
   ```
5. Check the endpoint (bash `curl`, PowerShell `curl.exe`):
   ```
   curl -s localhost:8080/api/leak
   curl -s localhost:8080/metrics | grep pprofdemo      # PowerShell: ... | Select-String pprofdemo
   ```
   Example output:
   ```
   pprofdemo_http_requests_in_flight 0
   pprofdemo_http_requests_total{code="200",method="GET",route="/api/leak"} 1
   pprofdemo_http_request_duration_seconds_bucket{method="GET",route="/api/leak",le="0.005"} 1
   pprofdemo_http_request_duration_seconds_sum{method="GET",route="/api/leak"} 0.000412
   ```

What is exported:

| Metric | Type | Labels |
|---|---|---|
| `pprofdemo_http_requests_total` | counter | method, route, code |
| `pprofdemo_http_request_duration_seconds` | histogram | method, route |
| `pprofdemo_http_requests_in_flight` | gauge | none |
| `go_*` (goroutines, heap, GC) and `process_*` (CPU, memory, fds) | runtime | none |

`route` is the route template (`/api/cpu`), never the raw URL, so labels stay bounded;
unknown paths are grouped as `unmatched`. Scrapes of `/metrics` itself are not counted.

> `/metrics` is on the public app port (8080). That is fine for a demo; in production
> restrict it (network policy, or serve it on the private pprof port 6060).

---

### Run the app from this folder (no pprof-demo needed)

`app-changes` has its own server with the CRUD API and `/metrics`
([app-changes/cmd/server/main.go](app-changes/cmd/server/main.go)):

```
cd prometheus/app-changes
go run ./cmd/server
```
It listens on `:8080`, the port both Prometheus configs scrape. Run **either** this
or `pprof-demo`, not both (same port). To use another port: `APP_ADDR=:9000 go run ./cmd/server`
(PowerShell: `$env:APP_ADDR=":9000"; go run ./cmd/server`) and change the target in the
Prometheus config. This server has no pprof endpoints and none of the `/api/cpu`, `/api/leak`
style endpoints; the goroutine, heap and CPU panels still work, but only the CRUD and
HTTP panels respond to your requests.

## 2A. Run with Docker

Requires Docker Desktop (Windows/Mac) or Docker Engine + Compose plugin.

```
# terminal 1: the app runs on the host
cd pprof-demo
go run ./cmd/server

# terminal 2: Prometheus + Grafana
cd prometheus/docker
docker compose up -d
docker compose ps
```

1. Prometheus http://localhost:9090/targets: `pprof-demo` must be **UP**.
2. Grafana http://localhost:3000 (admin / admin): the **pprof-demo** dashboard is the
   home page. The data source and dashboard are provisioned, nothing to click.

Day-to-day:

```
docker compose logs -f prometheus          # watch scrapes / rule errors
docker compose restart prometheus          # after editing docker/prometheus.yml
curl -X POST localhost:9090/-/reload       # reload config + alerts.yml without restart
docker compose down                        # stop, keep data
docker compose down -v                     # stop and delete stored metrics/dashboards
```

The container reaches the app through `host.docker.internal` (built in on Docker
Desktop, mapped by `extra_hosts` on Linux). If the app uses another port, edit the
target in `docker/prometheus.yml`.

---

## 2B. Run without Docker

1. Download and unzip
   - **Prometheus**: https://prometheus.io/download/ (`prometheus.exe` on Windows)
   - **Grafana OSS**: https://grafana.com/grafana/download
2. Start the app: `cd pprof-demo && go run ./cmd/server`
3. Start Prometheus **from `prometheus/local`** (the config uses the relative path `../alerts.yml`):

   bash:
   ```
   cd prometheus/local
   /path/to/prometheus --config.file=prometheus.yml
   ```
   PowerShell:
   ```
   cd prometheus\local
   C:\tools\prometheus\prometheus.exe --config.file=prometheus.yml
   ```
   Open http://localhost:9090/targets: `pprof-demo` must be **UP**.
4. Give Grafana its data source, then start it:

   bash:
   ```
   cp prometheus/local/grafana-datasource.yml /path/to/grafana/conf/provisioning/datasources/
   /path/to/grafana/bin/grafana server --homepath /path/to/grafana
   ```
   PowerShell:
   ```
   Copy-Item prometheus\local\grafana-datasource.yml C:\tools\grafana\conf\provisioning\datasources\
   C:\tools\grafana\bin\grafana.exe server --homepath C:\tools\grafana
   ```
   Keep `uid: prometheus` in that file; the dashboard refers to it.
5. Open http://localhost:3000 (admin / admin) → **Dashboards → New → Import** →
   upload `prometheus/grafana/dashboards/pprof-demo.json` → Import.

---

## 3. Generate traffic and watch the graphs

bash:
```
for i in $(seq 1 150); do curl -s -o /dev/null localhost:8080/api/leak; done        # heap grows
for i in $(seq 1 150); do curl -s -o /dev/null localhost:8080/api/goroutine; done   # goroutines grow
for i in $(seq 1 5);   do curl -s -o /dev/null "localhost:8080/api/cpu?n=38"; done  # CPU + latency
for i in $(seq 1 150); do curl -s -o /dev/null "localhost:8080/api/leak?fixed=true"; done  # flat
```
PowerShell:
```
1..150 | ForEach-Object { curl.exe -s -o NUL http://localhost:8080/api/leak }
1..150 | ForEach-Object { curl.exe -s -o NUL http://localhost:8080/api/goroutine }
1..5   | ForEach-Object { curl.exe -s -o NUL "http://localhost:8080/api/cpu?n=38" }
1..150 | ForEach-Object { curl.exe -s -o NUL "http://localhost:8080/api/leak?fixed=true" }
```
(`scripts/load.sh` in pprof-demo does the same with more options.)

| Dashboard panel | Moved by | Then take this pprof profile |
|---|---|---|
| Heap in use | `/api/leak` | `/debug/pprof/heap` |
| Goroutines | `/api/goroutine` | `/debug/pprof/goroutine` |
| Allocation rate | `/api/alloc` | `/debug/pprof/allocs` |
| p95 latency, CPU | `/api/cpu` | `/debug/pprof/profile?seconds=30` |
| p95 latency (lock wait) | `/api/lock` | `/debug/pprof/mutex` |
| 5xx error ratio | `/api/leak` after the 512 MiB cap (507 is a 5xx) | none |

## 3b. CRUD example with Prometheus metrics

The CRUD code lives in this folder as [app-changes/items](app-changes/items) (run its tests with
`cd prometheus/app-changes && go test ./...`) and is the same code the app runs from
[pprof-demo/internal/items](../pprof-demo/internal/items): `store.go` (thread-safe CRUD),
`handler.go` (Gin routes + metrics), `handler_test.go`. Data is lost when the app restarts.
If you edit one copy, copy the change to the other.

| Operation | Request | Success | Errors |
|---|---|---|---|
| Create | `POST /api/items` | 201 | 400 bad body |
| List | `GET /api/items` | 200 | none |
| Read | `GET /api/items/:id` | 200 | 400 bad id, 404 |
| Update | `PUT /api/items/:id` | 200 | 400, 404 |
| Delete | `DELETE /api/items/:id` | 204 | 400, 404 |

Body for create/update: `{"name":"pen","price":1.5,"quantity":10}`. `name` is required,
`price` and `quantity` must be >= 0.

bash:
```
H='Content-Type: application/json'
curl -s -X POST   localhost:8080/api/items   -H "$H" -d '{"name":"pen","price":1.5,"quantity":10}'
curl -s           localhost:8080/api/items
curl -s           localhost:8080/api/items/1
curl -s -X PUT    localhost:8080/api/items/1 -H "$H" -d '{"name":"pencil","price":2,"quantity":5}'
curl -s -X DELETE localhost:8080/api/items/1 -w "%{http_code}\n"
curl -s localhost:8080/api/items/99          # 404, counted as result="not_found"
```
PowerShell (single quotes around JSON, `curl.exe` is required):
```
curl.exe -s -X POST localhost:8080/api/items -H "Content-Type: application/json" -d '{\"name\":\"pen\",\"price\":1.5,\"quantity\":10}'
curl.exe -s localhost:8080/api/items
curl.exe -s localhost:8080/api/items/1
curl.exe -s -X PUT localhost:8080/api/items/1 -H "Content-Type: application/json" -d '{\"name\":\"pencil\",\"price\":2,\"quantity\":5}'
curl.exe -s -X DELETE localhost:8080/api/items/1 -w "%{http_code}"
```
Or, in PowerShell without quoting trouble:
```
Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/items -ContentType application/json -Body (@{name="pen";price=1.5;quantity=10} | ConvertTo-Json)
```

Metrics added (on the same `/metrics`):

| Metric | Type | Labels | Meaning |
|---|---|---|---|
| `pprofdemo_item_operations_total` | counter | operation (create, list, get, update, delete), result (success, invalid, not_found) | business outcome of each call |
| `pprofdemo_items_stored` | gauge | none | items currently in the store |

Example after a few calls:
```
pprofdemo_item_operations_total{operation="create",result="success"} 2
pprofdemo_item_operations_total{operation="create",result="invalid"} 1
pprofdemo_item_operations_total{operation="get",result="not_found"} 1
pprofdemo_items_stored 1
```
Every operation/result series exists from startup at 0, so dashboards never show "no data".
The generic HTTP metrics (`pprofdemo_http_requests_total{route="/api/items/:id"}`) also cover these
routes, with the route template as label so item IDs never create new series.

The dashboard has two panels for this (*Items CRUD operations/s* and *Items stored*). Try:
```promql
sum by (operation, result) (rate(pprofdemo_item_operations_total[1m]))           # ops/s
sum(rate(pprofdemo_item_operations_total{result!="success"}[5m]))                # failed ops/s
pprofdemo_items_stored                                                           # current count
```

To add the same to another Gin service: copy `app-changes/items` to `internal/items`, then in `main.go`
```go
items.NewHandler(items.NewStore(), m.Registerer()).Register(r)
```
`m.Registerer()` (in `metrics.go`) is what puts the item metrics on the same `/metrics` endpoint.

## 4. Useful queries

Paste into http://localhost:9090/graph:

```promql
sum by (route) (rate(pprofdemo_http_requests_total[1m]))                   # req/s per route
histogram_quantile(0.95, sum by (le, route)
  (rate(pprofdemo_http_request_duration_seconds_bucket[1m])))             # p95 per route
sum(rate(pprofdemo_http_requests_total{code=~"5.."}[1m]))
  / sum(rate(pprofdemo_http_requests_total[1m]))                          # error ratio
go_goroutines{job="pprof-demo"}                                           # goroutines now
deriv(go_memstats_heap_inuse_bytes{job="pprof-demo"}[2m])                 # heap growth, bytes/s
rate(go_memstats_alloc_bytes_total{job="pprof-demo"}[1m])                 # allocation rate
```

## 5. Alerts

Defined in [alerts.yml](alerts.yml), visible at http://localhost:9090/alerts.
Prometheus sends firing alerts to **Alertmanager** (section 6), which routes them to receivers.

| Alert | Fires when | Reproduce |
|---|---|---|
| AppDown | scrape fails for 15s | stop the app |
| GoroutineLeak | > 100 goroutines and rising for 1m | 150× `/api/goroutine` |
| HeapGrowing | heap in use grows > 100 KB/s for 1m | steady stream of `/api/leak` |
| HighLatencyP95 | p95 > 1s for 1m | keep calling `/api/cpu?n=40` |
| HighErrorRate | > 5% 5xx for 1m | `/api/leak` past the memory cap |

Validate rule edits without starting anything:
```
docker run --rm --entrypoint promtool -v "${PWD}:/p:ro" prom/prometheus:v2.55.1 check rules /p/alerts.yml
```
(In Git Bash prefix with `MSYS_NO_PATHCONV=1` and use `$(pwd -W)`.)

## 6. Alertmanager: alert routing

```
alerts.yml rule fires ──► Prometheus ──► Alertmanager ──► receiver (webhook / Slack / email)
                          (evaluates)    (group, route,    
                                          silence, dedupe)
```
Prometheus only decides *that* an alert fires. Alertmanager decides *who is told, how often, and what is suppressed*.

**What was changed to add it**

| File | Change |
|---|---|
| `docker/prometheus.yml`, `local/prometheus.yml` | new `alerting:` block pointing at Alertmanager (`alertmanager:9093` / `localhost:9093`) |
| `docker/docker-compose.yml` | new `alertmanager` service (port 9093, persistent volume) |
| `docker/alertmanager.yml`, `local/alertmanager.yml` | routing tree, inhibit rule, receivers |
| `alerts.yml` | already had a `severity` label on every rule, which the routing uses |
| `app-changes/cmd/webhook/main.go` | demo receiver that prints alerts |

**Routing in `alertmanager.yml`**

| Alert label | Receiver (webhook path) | Notes |
|---|---|---|
| `severity="critical"` (AppDown, HighErrorRate) | `/critical` | first notification after 5s, repeated every 15m |
| `severity="warning"` (GoroutineLeak, HeapGrowing, HighLatencyP95) | `/warning` | |
| anything else | `/default` | |

- `group_by: [alertname, job]` puts alerts of the same kind in one notification.
- `group_wait` / `group_interval` / `repeat_interval` control how soon and how often you are notified.
- The **inhibit rule** hides warnings for a `job` while a critical alert exists for the same job.
- `send_resolved: true` also notifies when the alert clears.

**Try it (Docker)**

```
# terminal 1: webhook receiver (prints alerts it gets)
cd prometheus/app-changes
go run ./cmd/webhook

# terminal 2: the app
cd prometheus/app-changes
go run ./cmd/server

# terminal 3: Prometheus + Alertmanager + Grafana
cd prometheus/docker
docker compose up -d
```
1. Stop the app (Ctrl+C in terminal 2).
2. After ~15s `AppDown` fires: http://localhost:9090/alerts (red) then http://localhost:9093 shows it.
3. About 5s later the webhook terminal prints, for example:
   ```
   [firing] route=/critical receiver=critical alerts=1
       firing   AppDown         severity=critical pprof-demo is not reachable
   ```
4. Start the app again; after a short delay a `[resolved]` message follows.

Other useful views: http://localhost:9093/#/alerts (active alerts), `#/silences` (mute an alert for a while),
http://localhost:9090/status (shows the Alertmanager Prometheus talks to).

**Run without Docker:** download Alertmanager from https://prometheus.io/download/, then from `prometheus/local`:
```
<path-to>\alertmanager.exe --config.file=alertmanager.yml
<path-to>\prometheus.exe --config.file=prometheus.yml
```
Start the webhook receiver the same way as above.

**Send to Slack or email instead:** in `alertmanager.yml` replace a `webhook_configs` block with the
commented `slack_configs` / `email_configs` example at the bottom of the file, put in your own URL or
SMTP details, and run `docker compose restart alertmanager` (or `curl -X POST localhost:9093/-/reload`).
Keep secrets out of Git: use `api_url_file` / `auth_password_file` for real deployments.

**Validate edits without starting anything**
```
docker run --rm --entrypoint amtool -v "${PWD}/docker:/c:ro" prom/alertmanager:v0.27.0 check-config /c/alertmanager.yml
```
Test where an alert would go: `amtool config routes test --config.file=docker/alertmanager.yml severity=critical alertname=AppDown`.

## Troubleshooting

| Symptom | Fix |
|---|---|
| Alert is **FIRING** in Prometheus but nothing in the webhook | Is `go run ./cmd/webhook` running on `:5001`? Check `docker compose logs alertmanager` for `connection refused`. |
| http://localhost:9090/status shows no Alertmanager | `alerting:` block missing or wrong target; run `docker compose restart prometheus`. |
| Alert shows in Alertmanager but notification is late | `group_wait` / `group_interval` delay it by design (5-10s / 1m here). |
| Warning alert never notifies while the app is down | The inhibit rule is hiding it behind the critical `AppDown`. |
| `Alertmanager` container exits | Run `amtool check-config` (above); YAML errors show in `docker compose logs alertmanager`. |
| `pprof-demo` target **DOWN** (Docker) | App must run on the host on `:8080`. Test `curl localhost:8080/metrics`. Windows firewall may prompt the first time; allow it. |
| `connection refused` to `host.docker.internal` on Linux | Keep the `extra_hosts` line in `docker-compose.yml`. |
| Dashboard shows **No data** | Send some traffic (section 3); confirm the data source uid is `prometheus`. |
| Latency panels empty / `NaN` | Normal until requests arrive inside the last minute. |
| Rules not loaded (no-Docker) | Start Prometheus from `prometheus/local`, or fix the `rule_files` path. |
| `port is already allocated` | Change the left side of `ports:` (e.g. `"9091:9090"`). |
| Counters reset to 0 | The app restarted; `rate()` handles this, so graphs recover. |
| IDE: *cannot find package prometheus* | Run `go mod tidy` in the folder that owns the file (`pprof-demo` or `prometheus/app-changes`), then restart the Go language server (Command Palette → *Go: Restart Language Server*). |
