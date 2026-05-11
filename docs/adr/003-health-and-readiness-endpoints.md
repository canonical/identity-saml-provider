# ADR 003: Health and Readiness Endpoints for Kubernetes Probes

## Status

Proposed

## Context

The Identity SAML Provider runs as a workload inside a
Juju Kubernetes charm. Kubernetes uses **liveness** and
**readiness** probes to determine whether a pod should be
restarted or removed from service traffic:

- **Liveness probe**: If this fails, the kubelet restarts
  the container. It detects fatal states such as
  deadlocks or unrecoverable panics.
- **Readiness probe**: If this fails, the pod's IP is
  removed from Service endpoints. No traffic is routed
  to the pod until it passes again. It detects transient
  unavailability such as a lost database connection.

The application currently exposes no dedicated health
endpoints. Without probes, Kubernetes can only check
whether the process is running — it cannot detect a
scenario where the HTTP server is up but the database
connection pool is exhausted, or where OIDC discovery
has become unreachable.

Additionally, the Juju charm layer needs well-defined
endpoints to configure `LivenessProbe` and
`ReadinessProbe` on the workload container.

## Decision

### 1. Add a liveness endpoint at `GET /healthz`

The liveness endpoint performs **no dependency checks**.
It returns `200 OK` immediately if the HTTP server is
able to handle requests:

```text
GET /healthz

200 OK
Content-Type: application/json

{"status": "ok"}
```

Rationale for keeping it trivial:

- A liveness probe that checks dependencies (e.g., DB)
  risks **cascading restarts**. If the database goes
  down, every pod would fail its liveness probe, get
  restarted simultaneously, and potentially storm the
  database during reconnection — making the outage worse.
- The only question liveness should answer is: "Is the
  process stuck?" If the HTTP handler can execute and
  write a response, the process is not stuck.

### 2. Add a readiness endpoint at `GET /readyz`

The readiness endpoint checks that the application can
serve real requests by verifying critical dependencies:

| Check      | Method                 | Failure meaning                          |
|------------|------------------------|------------------------------------------|
| PostgreSQL | `pool.Ping(ctx)`       | Cannot read/write sessions or SP records |

When all checks pass:

```text
GET /readyz

200 OK
Content-Type: application/json

{"status": "ready"}
```

When any check fails:

```text
GET /readyz

503 Service Unavailable
Content-Type: application/json

{"status": "not ready", "checks": {"postgres": "fail"}}
```

#### Why only PostgreSQL?

- **PostgreSQL** is the only stateful dependency that can
  become transiently unavailable at runtime. The
  connection pool may drain, the pod may lose network
  access to the database, or PostgreSQL may be
  temporarily restarting.
- **Ory Hydra** (OIDC provider) is checked during
  startup (`hydra.DiscoverOIDC`). If Hydra is
  unreachable at boot, the application fails to start
  entirely — `Build()` returns an error before the HTTP
  server is created. At runtime, Hydra errors manifest
  as OIDC callback failures for individual users, not as
  a systemic inability to serve any request. Including
  Hydra in the readiness check would couple pod
  availability to an external service, risking
  unnecessary traffic removal when Hydra experiences
  transient latency.
- **Pending requests** are stored in-memory
  (`memory.NewPendingRequestRepo`). There is no
  meaningful "ping" for an in-memory map.

### 3. Endpoint naming convention

The endpoints follow the `/healthz` and `/readyz`
convention established by the Kubernetes API server and
widely adopted in the Go ecosystem (e.g., `grpc-health`,
`k8s.io/apiserver`, `heptio/healthcheck`). The `-z`
suffix is a Kubernetes community convention with no
semantic meaning.

### 4. Implementation details

#### Registration

The endpoints will be registered in the chi router
alongside existing routes, within `handlers.RegisterRoutes`
in `internal/handler/`. They will be placed **before**
middleware that adds overhead (tracing, Prometheus
response time) to keep probes lightweight and avoid
polluting metrics with high-frequency probe traffic:

```go
// Health probes — no middleware, no auth
router.Get("/healthz", healthHandler.HandleHealthz)
router.Get("/readyz", healthHandler.HandleReadyz)
```

#### Handler layer

The handler methods will live in a new file
`internal/handler/health.go` with a dedicated
`HealthHandler` struct, separate from the existing
`Handlers` struct.

The readiness handler needs to ping PostgreSQL, but the
handler layer should not depend on `*pgxpool.Pool`
directly. Instead, a `Pinger` interface will be defined
in the handler package:

```go
// internal/handler/health.go

// Pinger abstracts the ability to check a dependency's
// connectivity. *pgxpool.Pool satisfies this interface.
type Pinger interface {
    Ping(ctx context.Context) error
}

// HealthHandler serves health and readiness probe
// endpoints.
type HealthHandler struct {
    pinger Pinger
}

func NewHealthHandler(pinger Pinger) *HealthHandler {
    return &HealthHandler{pinger: pinger}
}
```

`*pgxpool.Pool` already has a `Ping(context.Context) error`
method, so it satisfies this interface with no wrapper.

A separate `HealthHandler` struct was chosen over adding
the `Pinger` to the existing `Handlers` struct because:

- **`Handlers` is already large.** It has 10 fields and
  a 10-parameter constructor. Health endpoints use none
  of those dependencies — adding a `pinger` field would
  widen the struct and constructor for an unrelated
  concern.
- **Cohesion.** Health probes have no overlap with the
  SAML/OIDC/admin business logic. A dedicated struct
  keeps the health concern self-contained.
- **Test isolation.** `HealthHandler` tests only need a
  `Pinger` stub — no mock services, no noop monitor, no
  noop tracer. Existing test helpers for `Handlers` are
  untouched.
- **Independent evolution.** If new readiness checks are
  added later (e.g., a second dependency), only
  `HealthHandler` changes.

The `Pinger` interface approach:

- Keeps `*pgxpool.Pool` out of the handler package — no
  database driver import.
- Is trivially mockable in unit tests — a stub `Pinger`
  can return `nil` or an error.
- Follows the project's existing convention of depending
  on narrow interfaces rather than concrete types.

```go
func (h *HealthHandler) HandleHealthz(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *HealthHandler) HandleReadyz(w http.ResponseWriter, r *http.Request) {
    checks := map[string]string{}
    ready := true

    ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
    defer cancel()

    if err := h.pinger.Ping(ctx); err != nil {
        checks["postgres"] = "fail"
        ready = false
    } else {
        checks["postgres"] = "ok"
    }

    w.Header().Set("Content-Type", "application/json")
    if ready {
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status": "not ready",
            "checks": checks,
        })
    }
}
```

#### Timeout

The readiness handler uses a 2-second `context.WithTimeout`
for the database ping. This ensures the probe does not
hang indefinitely if PostgreSQL is unresponsive. The
Kubernetes probe configuration should set
`timeoutSeconds` to a value slightly higher (e.g., 3s)
to account for network overhead.

#### No authentication

Both endpoints must be **unauthenticated**. Kubernetes
kubelets call probes directly — they do not carry
application-level credentials.

#### Suggested Kubernetes probe configuration

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8082
  initialDelaySeconds: 5
  periodSeconds: 10
  timeoutSeconds: 3
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /readyz
    port: 8082
  initialDelaySeconds: 5
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 3
```

## Consequences

### Benefits

- **Automatic recovery**: Kubernetes restarts pods that
  are deadlocked (liveness) and stops sending traffic to
  pods that cannot reach the database (readiness).
- **Graceful dependency outages**: If PostgreSQL goes
  down temporarily, pods are removed from Service
  endpoints but **not restarted**, allowing them to
  recover in place once the database returns.
- **Observable**: The readiness response body includes
  per-check status, making it useful for debugging via
  `kubectl exec` or port-forward.
- **Lightweight**: The liveness probe has zero I/O cost.
  The readiness probe performs a single `Ping` with a
  tight timeout.
- **Charm-friendly**: The Juju charm can configure probes
  with well-known, stable paths.

### Drawbacks

- **Extra struct and wiring**: A new `HealthHandler`
  struct is introduced alongside the existing `Handlers`
  struct, and must be constructed and wired in
  `app.Build()`. This is a minor cost — the struct has
  one field and a trivial constructor.
- **Metric noise**: If probes are not excluded from
  middleware, high-frequency health check requests will
  inflate Prometheus metrics and access logs. The
  implementation must register probe routes before
  monitoring middleware, or explicitly exclude them.
