# Prometheus Setup Guide

Step-by-step setup of Prometheus (with Alertmanager and Grafana) to monitor the Go app in this repo.
For what each metric means, queries and alert details, see [README.md](README.md).

Commands are shown for **bash** (Git Bash, Linux, macOS) and **PowerShell**.
In PowerShell use `curl.exe`, not `curl`.

## 0. What you are building

```
Go app (:8080 /metrics) ◄── scrape every 5s ── Prometheus (:9090) ── alerts ──► Alertmanager (:9093) ──► webhook (:5001)
                                                     ▲
                                                     └── queries ── Grafana (:3000)
```

| Component | Port | Role |
|---|---|---|
| Go app | 8080 | exposes `GET /metrics` |
| Prometheus | 9090 | scrapes and stores metrics, evaluates `alerts.yml` |
| Alertmanager | 9093 | routes firing alerts |
| Webhook receiver (demo) | 5001 | prints alerts it receives |
| Grafana | 3000 | dashboards (admin / admin) |

## 1. Prerequisites

| Tool | Needed for | Check |
|---|---|---|
| Go 1.21+ | running the app | `go version` |
| Docker Desktop (or Docker Engine + Compose) | **Option A** (recommended) | `docker compose version` |
| Prometheus, Alertmanager, Grafana binaries | **Option B** (no Docker) only | see step 5B |

Make sure ports 8080, 9090, 9093, 5001 and 3000 are free.

## 2. Run the app and check `/metrics`

```
cd prometheus/app-changes
go run ./cmd/server
```

In a second terminal:

```
curl -s localhost:8080/api/items
curl -s localhost:8080/metrics | grep pprofdemo      # PowerShell: curl.exe -s localhost:8080/metrics | Select-String pprofdemo
```

You should see `pprofdemo_http_requests_total`, `pprofdemo_http_request_duration_seconds_*`
and `pprofdemo_http_requests_in_flight`. If not, stop here and fix the app first.

> Using the `pprof-demo` app instead? Run `cd pprof-demo && go run ./cmd/server`. Run one app, not both (same port 8080).

Leave the app running.

## 3. Understand the config (1 minute)

Prometheus is configured by `prometheus.yml`:

| Setting | Meaning |
|---|---|
| `scrape_interval: 5s` | how often `/metrics` is read |
| `rule_files: alerts.yml` | alert rules (shared by both options) |
| `alerting.alertmanagers` | where firing alerts are sent |
| `scrape_configs` job `pprof-demo` | the app target (`host.docker.internal:8080` in Docker, `localhost:8080` without) |

If your app uses another port, change the `targets` line in
[docker/prometheus.yml](docker/prometheus.yml) or [local/prometheus.yml](local/prometheus.yml).

## 4. Option A: Docker (recommended)

### Step 4.1: Start the stack

```
cd prometheus/docker
docker compose up -d
docker compose ps
```

All three services (`prometheus`, `alertmanager`, `grafana`) should be `running`.
The first run downloads the images (`prom/prometheus:v2.55.1`, `prom/alertmanager:v0.27.0`, `grafana/grafana:11.3.0`).

### Step 4.2: Verify (see section 6)

### Step 4.3: Day-to-day commands

```
docker compose logs -f prometheus          # watch scrapes and rule errors
docker compose restart prometheus          # after editing docker/prometheus.yml
curl -X POST localhost:9090/-/reload       # reload config + alerts.yml without restart
docker compose down                        # stop, keep data
docker compose down -v                     # stop and delete stored metrics and dashboards
```

The container reaches the app on the host through `host.docker.internal`
(built in on Docker Desktop, mapped by `extra_hosts` on Linux).

## 5. Option B: without Docker

### Step 5B.1: Download

| Tool | Download |
|---|---|
| Prometheus | https://prometheus.io/download/ |
| Alertmanager | https://prometheus.io/download/ |
| Grafana OSS | https://grafana.com/grafana/download |

Unzip each, for example to `C:\tools\prometheus`, `C:\tools\alertmanager`, `C:\tools\grafana`
(or `/opt/...` on Linux/macOS).

### Step 5B.2: Start Alertmanager

PowerShell:
```
cd prometheus\local
C:\tools\alertmanager\alertmanager.exe --config.file=alertmanager.yml
```
bash:
```
cd prometheus/local
/path/to/alertmanager --config.file=alertmanager.yml
```

### Step 5B.3: Start Prometheus

Start it **from `prometheus/local`** (the config uses the relative path `../alerts.yml`).

PowerShell:
```
cd prometheus\local
C:\tools\prometheus\prometheus.exe --config.file=prometheus.yml
```
bash:
```
cd prometheus/local
/path/to/prometheus --config.file=prometheus.yml
```

### Step 5B.4: Configure and start Grafana

PowerShell:
```
Copy-Item prometheus\local\grafana-datasource.yml C:\tools\grafana\conf\provisioning\datasources\
C:\tools\grafana\bin\grafana.exe server --homepath C:\tools\grafana
```
bash:
```
cp prometheus/local/grafana-datasource.yml /path/to/grafana/conf/provisioning/datasources/
/path/to/grafana/bin/grafana server --homepath /path/to/grafana
```
Keep `uid: prometheus` in that file; the dashboard refers to it.

### Step 5B.5: Import the dashboard

1. Open http://localhost:3000 (admin / admin).
2. **Dashboards → New → Import**.
3. Upload `prometheus/grafana/dashboards/pprof-demo.json` and click **Import**.

## 6. Verify the setup

Work through this checklist in order:

| # | Check | Expected |
|---|---|---|
| 1 | http://localhost:8080/metrics | metrics text is returned |
| 2 | http://localhost:9090/targets | job `pprof-demo` is **UP** |
| 3 | http://localhost:9090/graph, query `up` | value `1` for both jobs |
| 4 | http://localhost:9090/alerts | 5 rules listed (AppDown, GoroutineLeak, HeapGrowing, HighLatencyP95, HighErrorRate), all green |
| 5 | http://localhost:9090/status | Alertmanager listed under *Alertmanagers* |
| 6 | http://localhost:9093 | Alertmanager UI loads |
| 7 | http://localhost:3000 | **pprof-demo** dashboard shows data (send traffic first, step 7) |

## 7. Generate traffic

bash:
```
for i in $(seq 1 150); do curl -s -o /dev/null localhost:8080/api/items; done
```
PowerShell:
```
1..150 | ForEach-Object { curl.exe -s -o NUL http://localhost:8080/api/items }
```
Refresh Grafana; the request-rate and latency panels should move within about 10 seconds.
(`/api/leak`, `/api/goroutine`, `/api/cpu` exist only in `pprof-demo`; see the README for the full list.)

## 8. Test an alert end to end

```
# terminal 1: webhook receiver (prints alerts)
cd prometheus/app-changes
go run ./cmd/webhook
```

1. Stop the app (Ctrl+C in the app terminal).
2. After about 15s `AppDown` goes red at http://localhost:9090/alerts and appears at http://localhost:9093.
3. About 5s later the webhook terminal prints:
   ```
   [firing] route=/critical receiver=critical alerts=1
       firing   AppDown         severity=critical pprof-demo is not reachable
   ```
4. Start the app again; a `[resolved]` message follows.

## 9. Change the configuration

| Want to change | Edit | Then |
|---|---|---|
| Scrape target or interval | `docker/prometheus.yml` or `local/prometheus.yml` | `docker compose restart prometheus` or `curl -X POST localhost:9090/-/reload` |
| Alert rules | `alerts.yml` | `curl -X POST localhost:9090/-/reload` |
| Alert routing, Slack or email | `docker/alertmanager.yml` or `local/alertmanager.yml` | `docker compose restart alertmanager` or `curl -X POST localhost:9093/-/reload` |
| Dashboard | edit in Grafana, export JSON to `grafana/dashboards/pprof-demo.json` | restart Grafana (Docker) |

Validate edits before reloading (run from `prometheus/`):

```
docker run --rm --entrypoint promtool -v "${PWD}:/p:ro" prom/prometheus:v2.55.1 check rules /p/alerts.yml
docker run --rm --entrypoint amtool -v "${PWD}/docker:/c:ro" prom/alertmanager:v0.27.0 check-config /c/alertmanager.yml
```
(In Git Bash prefix with `MSYS_NO_PATHCONV=1` and use `$(pwd -W)`.)

## 10. Troubleshooting

| Symptom | Fix |
|---|---|
| Target `pprof-demo` **DOWN** (Docker) | App must run on the host on `:8080`; test `curl localhost:8080/metrics`. Allow the Windows firewall prompt. |
| `connection refused` to `host.docker.internal` on Linux | Keep the `extra_hosts` line in `docker-compose.yml`. |
| Rules not loaded (no Docker) | Start Prometheus from `prometheus/local`, or fix the `rule_files` path. |
| Dashboard shows **No data** | Send traffic (step 7); confirm the data source uid is `prometheus`. |
| `port is already allocated` | Change the left side of `ports:` (for example `"9091:9090"`). |
| Alert fires but webhook prints nothing | Is `go run ./cmd/webhook` running on `:5001`? Check `docker compose logs alertmanager`. |
| No Alertmanager at `/status` | `alerting:` block missing or wrong target; restart Prometheus. |
| Latency panels empty or `NaN` | Normal until requests arrive in the last minute. |

## 11. Clean up

```
cd prometheus/docker
docker compose down -v      # removes containers and stored data
```
Stop the app and webhook receiver with Ctrl+C. For Option B, stop the three binaries the same way.
