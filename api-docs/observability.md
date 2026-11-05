# Observability REST Endpoints — Health / Ready / Metrics

Every service exposes plain REST health + Prometheus metrics endpoints alongside
its GraphQL API. Use these from PowerShell / any HTTP client to verify a service
is alive, ready, and exposing metrics — no GraphQL or JWT needed.

All curls below assume the Docker Compose / local default ports.

> **Kubernetes (skaffold) note:** `skaffold dev` forwards every service port
> listed below to the same `localhost` port (see `portForward` in
> `infrastructure/skaffold/skaffold.yaml`), so **the same commands work
> unchanged** once skaffold is running. To verify in-cluster instead, run
> `kubectl run curl --rm -i --restart=Never --image=curlimages/curl:latest -- sh`
> and replace `http://localhost:PORT` with `http://<service-srv>:PORT`
> (e.g. `http://driver-srv:9105/metrics`).

---

## 1. API Gateway (port 4000 + liveness port 4099)

```powershell
# Basic ping (public)
curl http://localhost:4000/ping

# Health (process stats, always 200 if the process runs)
curl http://localhost:4000/health

# Prometheus metrics (text/plain, scrape format)
curl http://localhost:4000/metrics

# Dedicated liveness server (separate port 4099 — answers ANY path with 200)
curl http://localhost:4099/health
curl http://localhost:4099/health/ready
curl http://localhost:4099/health/live
```

> The liveness server on 4099 always returns `{ "status": "alive" }` — it is
> intentionally dumb so Kubernetes can tell "process hung" from "app unhealthy".

---

## 2. User Service (port 4001)

```powershell
# Liveness — always 200 while the process runs
curl http://localhost:4001/user/health

# Readiness — 503 if Postgres OR Redis is down (checks both)
curl http://localhost:4001/user/ready

# Prometheus metrics
curl http://localhost:4001/user/metrics
```

---

## 3. Delivery Service (port 4003)

The delivery service registers the same handlers under several alias paths
(both bare and prefixed) so probes and manual curls both work:

```powershell
# Health (aliases: /health, /delivery/health)
curl http://localhost:4003/health
curl http://localhost:4003/delivery/health

# Liveness (aliases: /health/live, /delivery/health/live, /delivery/live)
curl http://localhost:4003/health/live
curl http://localhost:4003/delivery/live

# Readiness (aliases: /health/ready, /delivery/health/ready, /delivery/ready)
curl http://localhost:4003/health/ready
curl http://localhost:4003/delivery/ready

# Prometheus metrics (aliases: /metrics, /delivery/metrics)
curl http://localhost:4003/metrics
curl http://localhost:4003/delivery/metrics
```

---

## 4. Notification Service (port 4004)

```powershell
# Liveness
curl http://localhost:4004/notification/health

# Readiness — 503 if Postgres OR Redis is down
curl http://localhost:4004/notification/ready

# Prometheus metrics
curl http://localhost:4004/notification/metrics
```

---

## 5. Realtime Service (port 4006)

The realtime health endpoint checks Redis, NATS, and Kafka and can report
`DEGRADED` (critical deps up, Kafka down) or `DOWN`:

```powershell
# Full health — checks redis + nats + kafka, may return DEGRADED
curl http://localhost:4006/realtime/health

# Readiness — 503 if Redis OR NATS is down (Kafka does not gate readiness)
curl http://localhost:4006/realtime/ready

# Prometheus metrics
curl http://localhost:4006/realtime/metrics
```

---

## 6. Payment Service (Go — port 4002, metrics 9106, webhook 4012)

```powershell
# Liveness
curl http://localhost:4002/health/live

# Readiness — pings the database
curl http://localhost:4002/health/ready

# Prometheus metrics (also served on the dedicated metrics port 9106)
curl http://localhost:4002/metrics
curl http://localhost:9106/metrics

# Stripe webhook receiver (separate server on 4012, expects Stripe-signed POST)
# curl -X POST http://localhost:4012/webhook/payment
```

---

## 7. Media Service (Go — port 4005, metrics 9102)

```powershell
# Healthz — simple 200 OK on the GraphQL port
curl http://localhost:4005/healthz

# Prometheus metrics — dedicated metrics port only
curl http://localhost:9102/metrics
```

---

## 8. Search Service (Go — port 4007, metrics 9103)

```powershell
# Liveness
curl http://localhost:4007/health/live

# Readiness — pings OpenSearch
curl http://localhost:4007/health/ready

# Alias healthz (same handler)
curl http://localhost:4007/healthz

# Prometheus metrics — dedicated metrics port only
curl http://localhost:9103/metrics
```

---

## 9. Driver Service (Go — port 4008, metrics 9105)

```powershell
# Liveness
curl http://localhost:4008/health/live

# Readiness — pings the database
curl http://localhost:4008/health/ready

# Alias healthz (same handler)
curl http://localhost:4008/healthz

# Prometheus metrics — dedicated metrics port (also serves /healthz)
curl http://localhost:9105/metrics
curl http://localhost:9105/healthz
```

---

## 10. Analytics Service (Go — port 4009, metrics 9107)

```powershell
# Liveness
curl http://localhost:4009/health/live

# Readiness — pings ClickHouse
curl http://localhost:4009/health/ready

# Prometheus metrics (also served on the dedicated metrics port 9107)
curl http://localhost:4009/metrics
curl http://localhost:9107/metrics
```

---

## 11. Observability Stack Endpoints

```powershell
# ---- Prometheus (9090) ----
curl http://localhost:9090/-/healthy
curl http://localhost:9090/-/ready
# Are all targets UP? (expect count == 13: 10 services + prometheus + rabbitmq + alertmanager)
curl "http://localhost:9090/api/v1/query?query=up"
# How many alert rules are loaded?
curl http://localhost:9090/api/v1/rules
# Are any alerts firing right now?
curl http://localhost:9090/api/v1/alerts

# ---- Alertmanager (9093) ----
curl http://localhost:9093/-/healthy
curl http://localhost:9093/-/ready
# List currently active alerts
curl http://localhost:9093/api/v2/alerts
# Fire a one-off test alert into the pipeline (manual smoke test)
curl -X POST http://localhost:9093/api/v2/alerts -H "Content-Type: application/json" -d '[{"labels":{"alertname":"ManualTest","severity":"warning"},"annotations":{"summary":"Manual smoke test alert"},"startsAt":"2026-10-12T00:00:00Z"}]'

# ---- Alertmanager-Discord Relay (8081 on host, 8080 in network) ----
# This relay converts Alertmanager payloads into Discord embeds.
# healthz
curl http://localhost:8081/healthz
# Send a test Alertmanager payload directly to the relay (appears in Discord as a colored embed)
curl -X POST http://localhost:8081/webhook -H "Content-Type: application/json" -d '{"receiver":"default-notifications","status":"firing","alerts":[{"status":"firing","labels":{"alertname":"RelayTest","severity":"critical"},"annotations":{"summary":"Relay smoke test"},"startsAt":"2026-10-12T00:00:00Z"}]}'

# ---- Grafana (3000, admin/admin by default) ----
curl http://localhost:3000/api/health
# List provisioned dashboards
curl -u admin:admin http://localhost:3000/api/search?type=dash-db

# ---- Loki (3100) ----
curl http://localhost:3100/ready
# Query recent logs (LogQL)
curl "http://localhost:3100/loki/api/v1/query_range?query={service=\"api-gateway\"}&limit=5"

# ---- Tempo (3200) ----
curl http://localhost:3200/ready

# ---- RabbitMQ management (15672, delivery/delivery-dev-password) ----
curl -u delivery:delivery-dev-password http://localhost:15672/api/healthchecks/node
# Prometheus-format RabbitMQ metrics
curl http://localhost:15692/metrics
```

---

## 12. Quick Smoke-Test Checklist

Run these in order to verify the whole observability pipeline end-to-end:

```powershell
# 1. Every service answers its liveness probe
curl http://localhost:4000/health
curl http://localhost:4001/user/health
curl http://localhost:4003/health
curl http://localhost:4004/notification/health
curl http://localhost:4006/realtime/health
curl http://localhost:4002/health/live
curl http://localhost:4005/healthz
curl http://localhost:4007/health/live
curl http://localhost:4008/health/live
curl http://localhost:4009/health/live

# 2. Every service exposes Prometheus metrics
curl http://localhost:4000/metrics
curl http://localhost:4001/user/metrics
curl http://localhost:4003/metrics
curl http://localhost:4004/notification/metrics
curl http://localhost:4006/realtime/metrics
curl http://localhost:4002/metrics
curl http://localhost:9102/metrics
curl http://localhost:9103/metrics
curl http://localhost:9105/metrics
curl http://localhost:9107/metrics
curl http://localhost:4009/metrics

# 3. Prometheus has scraped all targets
curl "http://localhost:9090/api/v1/query?query=count(up==1)"

# 4. Alertmanager is up and has no unexpected alerts
curl http://localhost:9093/api/v2/alerts

# 5. Fire a manual test alert and watch it appear in Alertmanager
curl -X POST http://localhost:9093/api/v2/alerts -H "Content-Type: application/json" -d '[{"labels":{"alertname":"SmokeTest","severity":"info"},"annotations":{"summary":"End-to-end pipeline smoke test"},"startsAt":"2026-10-12T00:00:00Z"}]'
curl http://localhost:9093/api/v2/alerts
```

> Note: Go services (media, search, driver) serve `/metrics` ONLY on their
> dedicated metrics ports (9102 / 9103 / 9105) — not on the main GraphQL port.
> Payment and analytics expose `/metrics` on BOTH the main port and the
> dedicated port.

---

## 13. Kubernetes (skaffold) In-Cluster Verification

When running via `skaffold dev`, the same checks can be run from inside the
cluster without port-forwarding. Start an interactive curl pod:

```powershell
kubectl run curl --rm -i --restart=Never --image=curlimages/curl:latest -- sh
```

Then, inside that shell, replace `http://localhost:PORT` with the ClusterIP
service DNS name (all services run in the `default` namespace):

```sh
# Every service answers its liveness probe
curl http://api-gateway-srv:4000/health
curl http://user-srv:4001/user/health
curl http://delivery-srv:4003/health
curl http://notification-srv:4004/notification/health
curl http://realtime-srv:4006/realtime/health
curl http://payment-srv:4002/health/live
curl http://media-srv:4005/healthz
curl http://search-srv:4007/health/live
curl http://driver-srv:4008/health/live
curl http://analytics-srv:4009/health/live

# Every service exposes Prometheus metrics (dedicated ports for Go services)
curl http://api-gateway-srv:4000/metrics
curl http://user-srv:4001/user/metrics
curl http://delivery-srv:4003/metrics
curl http://notification-srv:4004/notification/metrics
curl http://realtime-srv:4006/realtime/metrics
curl http://payment-srv:9106/metrics
curl http://media-srv:9102/metrics
curl http://search-srv:9103/metrics
curl http://driver-srv:9105/metrics
curl http://analytics-srv:9107/metrics

# Observability stack
curl http://prometheus-srv:9090/-/ready
curl http://alertmanager-srv:9093/-/ready
curl http://alertmanager-discord-srv:8080/healthz
curl http://grafana-srv:3000/api/health
curl http://loki-srv:3100/ready
curl http://tempo-srv:3200/ready
curl -u delivery:delivery-dev-password http://rabbitmq-srv:15672/api/healthchecks/node
```

To run these non-interactively (one-shot):

```powershell
Get-Content check-endpoints.sh -Raw | kubectl run curl --rm -i --restart=Never --image=curlimages/curl:latest -- sh
```

Or verify Prometheus scrape health from inside the cluster:

```sh
# How many targets are up? (expect 13: 10 services + prometheus + rabbitmq + alertmanager)
curl "http://prometheus-srv:9090/api/v1/query?query=count(up==1)"

# Which targets are down? (expect empty result [])
curl "http://prometheus-srv:9090/api/v1/query?query=up==0"

# Active alerts? (expect [] when healthy)
curl http://alertmanager-srv:9093/api/v2/alerts
```
