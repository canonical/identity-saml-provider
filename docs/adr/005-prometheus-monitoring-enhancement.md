# ADR 005: Prometheus Monitoring Enhancement

## Status

Proposed

## Context

The application exposes a Prometheus metrics endpoint
at `GET /metrics` via `promhttp.Handler()` in
`internal/handler/routes.go`. A Prometheus-backed
monitor is wired in `internal/app/app.go` through
`prometheus.NewMonitor(serviceName, logger)`.

Today the custom Prometheus instrumentation is minimal:

| Metric                       | Type      | Labels            | State             |
|------------------------------|-----------|-------------------|-----------------  |
| `http_response_time_seconds` | Histogram | `route`, `status` | Active            |
| `dependency_available`       | Gauge     | `component`       | Defined, not used |

Both metrics carry `service` as a constant label.
The default Prometheus registry also exposes standard
Go runtime and process collectors.

This ADR evaluates the monitoring gaps through the
lens of Google's four golden signals — latency,
traffic, errors, and saturation — and proposes a
focused enhancement that addresses each one.

### Golden signal coverage today

| Signal     | Current coverage                        |
|------------|-----------------------------------------|
| Latency    | Partial — histogram exists but uses a   |
|            | non-standard name and bakes the HTTP    |
|            | method into the `route` label           |
| Traffic    | None — no request counter exists        |
| Errors     | None — errors are only visible as a     |
|            | subset of the histogram's `status`      |
|            | label, which requires `histogram_count` |
|            | queries instead of a simple counter     |
| Saturation | None — no in-flight, pool, or queue     |
|            | metric exists                           |

### Additional gaps

1. **The `/metrics` endpoint is registered inside the
   middleware-wrapped route group** in `app.go`.
   Prometheus scrapes currently pass through request
   logging, tracing, and response-time middleware.
   This creates a feedback loop: the act of scraping
   changes the very series being scraped.

2. **The `dependency_available` gauge is dead code.**
   The `MonitorInterface` declares
   `SetDependencyAvailability`, the Prometheus
   implementation registers the gauge, but no code in
   the active handler/service layer calls it. The
   legacy `internal/provider/` package used to call it
   for Hydra during token exchange, but the refactored
   code in `internal/handler/` and `internal/service/`
   does not.

3. **The monitoring interface is untyped.** Methods
   accept `map[string]string` tag maps, making it easy
   to introduce unbounded label values (request IDs,
   email addresses, entity IDs) by accident.

4. **No direct test coverage** exists under
   `internal/monitoring/` or
   `internal/monitoring/prometheus/`.

## Decision

We will enhance Prometheus monitoring so that each
golden signal has at least one dedicated metric. Every
decision below is evaluated by asking: does this help
answer latency, traffic, errors, or saturation
questions?

### 1. HTTP server metrics — latency, traffic, errors

We will expose two HTTP metrics that together cover
three golden signals:

| Metric                                 | Type      | Labels                                 | Signal            |
|----------------------------------------|-----------|----------------------------------------|-------------------|
| `http_server_request_duration_seconds` | Histogram | `service`, `method`, `route`, `status` | Latency + traffic |
| `http_server_requests_total`           | Counter   | `service`, `method`, `route`, `status` | Traffic + errors  |

These replace the current `http_response_time_seconds`.
The project is early enough that adopting conventional
metric names now avoids carrying a non-standard name
forward.

Splitting `method` out of `route` into its own label
makes aggregation ergonomic:

```text
# error rate per route, any method
sum(rate(http_server_requests_total{status=~"5.."}[5m]))
  by (route)
  /
sum(rate(http_server_requests_total[5m])) by (route)
```

The `route` label will continue to use the chi route
pattern so that dynamic path segments do not create
unbounded cardinality.

#### Why no in-flight gauge

This ADR does not propose an
`http_server_in_flight_requests` gauge for two
reasons:

1. **Technical limitation.** Chi resolves the route
   pattern during `next.ServeHTTP()`. A middleware
   that increments a gauge before calling `next` does
   not yet know `route`. An in-flight gauge without
   `route` (only `service`) is a single global number
   that is less useful than a direct saturation signal
   like connection pool utilisation.

2. **Weak saturation signal.** In-flight counts how
   many goroutines are handling HTTP requests. For a
   bridge service that spends most of its time waiting
   on Hydra and PostgreSQL, goroutine count says
   little about how close the service is to a resource
   limit. The pgxpool connection pool is a more direct
   saturation signal (see Decision 3).

If in-flight tracking is needed later, it can be added
as a simple global gauge without `route`.

#### Histogram bucket configuration

The application has a bimodal latency profile:

- Fast operations: `/saml/metadata` and
  `/admin/service-providers` typically complete in
  under 10 ms.
- Slow operations: `/saml/callback` performs an OIDC
  code exchange with Hydra plus a database write,
  typically completing in 100 ms to 1 s.

The default Prometheus histogram buckets
(`.005 .01 .025 .05 .1 .25 .5 1 2.5 5 10`)
provide reasonable resolution for both profiles. We
will use the defaults initially and adjust only if
production percentile queries show insufficient bucket
granularity.

### 2. Bridge operation counter — errors

HTTP status codes tell operators that a request failed,
but `/saml/callback` performs three sequential steps:

1. OIDC code exchange with Hydra
2. Session creation (database write)
3. Pending request retrieval (in-memory lookup)

A 502 on `/saml/callback` could mean Hydra is down or
the database is unreachable. A 500 could mean the
session save failed or the pending request was
corrupted. The HTTP counter alone cannot disambiguate.

We will expose one counter for bridge-specific error
attribution:

| Metric                    | Type    | Labels                           | Signal |
|---------------------------|---------|----------------------------------|--------|
| `bridge_operations_total` | Counter | `service`, `operation`, `result` | Errors |

The `operation` label uses a closed set:

- `oidc_code_exchange`
- `session_create`
- `pending_request_retrieve`

The `result` label uses two values:

- `success`
- `error`

Error classification (upstream vs validation vs
not-found) is already carried by the HTTP status code
on the request counter and by structured log fields.
Duplicating that taxonomy into a Prometheus label would
create a maintenance burden with little additional
signal.

#### What is excluded and why

The following operations are **not** tracked by the
bridge counter because they map 1:1 to an HTTP
endpoint and are fully described by the HTTP metrics:

| Operation                       | Why excluded                 |
|---------------------------------|------------------------------|
| `service_provider_registration` | 1:1 with `POST /admin/sps`   |
| `pending_request_store`         | 1:1 with `GET /saml/sso`     |

If an operator wants to know the SP registration
error rate, they query:

```text
rate(http_server_requests_total{
  route="/admin/service-providers",
  status=~"4..|5.."
}[5m])
```

No additional counter is needed.

### 3. Connection pool metrics — saturation

The fourth golden signal — saturation — asks: how
close is the service to a resource limit?

For this application the binding constraint is the
pgxpool connection pool. Every SAML flow touches
PostgreSQL (session read/write, SP lookup). If the
pool is exhausted, requests block on
`pool.Acquire()` and latency spikes until timeouts
fire.

`pgxpool.Pool.Stat()` exposes pool internals. We will
export the following gauges, collected at scrape time
via a custom Prometheus collector:

| Metric                         | Source                   | Purpose                      |
|--------------------------------|--------------------------|------------------------------|
| `db_pool_acquired_connections` | `Stat().AcquiredConns()` | Connections currently in use |
| `db_pool_idle_connections`     | `Stat().IdleConns()`     | Connections available        |
| `db_pool_total_connections`    | `Stat().TotalConns()`    | Current pool size            |
| `db_pool_max_connections`      | `Stat().MaxConns()`      | Configured pool limit        |

All are gauges and carry `service` as a constant
label.

The saturation ratio is:

```text
db_pool_acquired_connections
  / db_pool_max_connections
```

When this ratio approaches 1, the service is
saturated. Operators can alert on this ratio or on
`db_pool_idle_connections == 0`.

This is preferred over an in-flight HTTP gauge because
it measures the actual bottleneck rather than a proxy.

### 4. Drop `dependency_available` — not a golden signal

The existing `dependency_available` gauge and the
`SetDependencyAvailability` interface method will be
removed rather than wired up.

#### Why not monitor PostgreSQL availability

PostgreSQL availability is already monitored by two
mechanisms:

1. **Readiness probe.** `GET /readyz` pings Postgres
   every 5 seconds. If Postgres is unreachable,
   Kubernetes removes the pod from service endpoints.
   This is automatic and requires no Prometheus query.

2. **HTTP error rate.** A dead database causes session
   saves and SP lookups to fail, producing 5xx
   responses visible in `http_server_requests_total`.

A `dependency_available{component="postgres"}` gauge
would be a third, slower, and weaker signal for the
same condition. It also requires a background
health-check goroutine with lifecycle management
(context cancellation, graceful shutdown) and creates
additional periodic load on Postgres — all for
information already available.

#### Why not monitor Hydra availability

Hydra is validated at startup — if OIDC discovery
fails, `app.Build()` returns an error and the process
does not start. At runtime, Hydra failures are
per-request events that surface as `ErrUpstream` →
HTTP 502 on `/saml/callback`. This is visible in:

```text
rate(http_server_requests_total{
  route="/saml/callback",
  status="502"
}[5m])
```

and in the bridge counter:

```text
rate(bridge_operations_total{
  operation="oidc_code_exchange",
  result="error"
}[5m])
```

A boolean availability gauge adds no information
beyond what these counters already provide, and it
cannot distinguish transient per-request errors from
systemic outages — that distinction requires rate-based
analysis, which works directly on the counters.

#### Why not monitor dependency latency or internals

PostgreSQL and Hydra should be monitored by their own
exporters:

- **PostgreSQL**: the `postgres_exporter` exposes
  query latency, connection counts, replication lag,
  lock waits, and hundreds of other metrics. The SAML
  provider cannot replicate this depth, and attempting
  to do so would duplicate effort.
- **Hydra**: Ory Hydra exposes its own `/metrics`
  endpoint with request latency, token exchange
  counts, and internal state.

The SAML provider's job is to report its own golden
signals. If Hydra or Postgres is slow, that surfaces
in the provider's request duration histogram. If
Hydra or Postgres is down, that surfaces in the
provider's error counter. Instrumenting outbound HTTP
clients or database queries within this application
would add complexity without providing signals that
the dedicated exporters do not already offer better.

The pgxpool connection pool metrics (Decision 3) are
the one exception. They describe the provider's own
resource consumption, not Postgres internals. The
pool lives in this process, and its saturation is not
visible to any external exporter.

### 5. Exclude `/metrics` from custom middleware

The `/metrics` endpoint will be registered outside the
middleware-wrapped route group, alongside `/healthz`
and `/readyz`. This prevents Prometheus scrapes from
generating request logs, trace spans, and latency
histogram observations.

The rationale follows the same logic already applied
to health probes: operational infrastructure traffic
should not pollute application-level signals.

### 6. Standardise label cardinality rules

Prometheus labels must stay low-cardinality and
stable. The following values are forbidden as label
values:

- Request IDs, trace IDs
- SAML request IDs, relay state values
- Entity IDs, ACS URLs
- Email addresses, user subjects, usernames
- Raw query strings, full error messages

These belong in logs and traces. This policy will be
documented in a code comment in the monitoring
package.

### 7. Make the monitoring interface typed

The `MonitorInterface` currently uses untyped
`map[string]string` parameters:

```go
type MonitorInterface interface {
    GetService() string
    SetResponseTimeMetric(
        map[string]string, float64,
    ) error
    SetDependencyAvailability(
        map[string]string, float64,
    ) error
}
```

This will evolve to explicit, typed methods that map
to the metric families defined above:

```go
type MonitorInterface interface {
    ObserveHTTPRequestDuration(
        method, route, status string,
        durationSeconds float64,
    )
    IncrementHTTPRequestsTotal(
        method, route, status string,
    )
    IncrementBridgeOperation(
        operation, result string,
    )
}
```

`GetService()` is removed — the service name is a
construction-time constant, not a runtime query.
`SetDependencyAvailability` is removed per Decision 4.

The noop implementation, the Prometheus
implementation, and all call sites will be updated.
This change aligns with ADR 002's decision to migrate
all packages to the `logging.Logger` interface — the
monitoring interface update can be coordinated with
the logger migration.

Connection pool metrics (Decision 3) use a Prometheus
`Collector` registered at construction time. They are
collected at scrape time by Prometheus, not at request
time by application code, so they do not flow through
this interface.

### 8. Add monitoring tests

Direct tests will cover:

- HTTP middleware label generation and route exclusion
- Metric registration and re-registration behaviour
- Bridge operation counter increments
- Connection pool collector output

Tests will use a local Prometheus registry
(`prometheus.NewRegistry()`) to avoid global state
leaks between test cases.

### 9. Keep scrape configuration out of scope

The application exposes `/metrics` on the main HTTP
port. This ADR does not add a `ServiceMonitor` or
`prometheus.io/scrape` annotations to `k8s/`, because
scrape configuration is deployment-specific. The ADR
will document the scrape contract (port, path,
protocol) so overlays, charms, or cluster operators
can configure discovery appropriately.

## Consequences

### Benefits

- **All four golden signals covered.** Latency,
  traffic, errors, and saturation each have a
  dedicated, queryable metric.
- **Error attribution in multi-step flows.** The
  bridge operation counter disambiguates which step
  of the callback flow failed, information that HTTP
  status codes alone cannot provide.
- **Direct saturation signal.** pgxpool connection
  pool gauges measure the actual bottleneck, not a
  proxy. Operators can alert before pool exhaustion
  causes user-visible latency.
- **Less dead code.** Removing `dependency_available`
  and `SetDependencyAvailability` eliminates
  instrumentation that was never wired in the active
  code path.
- **Cleaner metrics.** Excluding `/metrics` from
  middleware avoids self-scrape noise.
- **Safer labels.** Typed interface methods and a
  documented cardinality policy reduce the risk of
  metric explosion.
- **Testable contract.** Monitoring behaviour becomes
  part of the automated regression surface.

### Drawbacks

- **Metric rename.** Replacing
  `http_response_time_seconds` with
  `http_server_request_duration_seconds` requires
  updating any dashboards or alert rules that consume
  the old name. Given the project's maturity, this
  cost is low.
- **pgxpool collector coupling.** The pool collector
  depends on `pgxpool.Stat()`. If the database driver
  changes, the collector must be rewritten. This is
  acceptable because the pool is a stable, long-term
  dependency.
- **More instrumentation call sites.** Service-layer
  code for the callback flow will need
  `IncrementBridgeOperation` calls. This is a small
  incremental cost per call site.
- **Interface migration.** Changing `MonitorInterface`
  from untyped maps to typed methods touches every
  consumer (handlers, middleware, tests). This is
  mechanical and can be coordinated with ADR 002's
  logger migration.
