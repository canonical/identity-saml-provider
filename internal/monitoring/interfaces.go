package monitoring

//go:generate mockgen -destination=../../mocks/mock_monitor.go -package=mocks . MonitorInterface

// MonitorInterface defines the application-level metrics contract.
// Every method corresponds to a specific Prometheus metric family.
//
// Label cardinality rules — the following values are FORBIDDEN as
// label values because they create unbounded series:
//   - Request IDs, trace IDs
//   - SAML request IDs, relay state values
//   - Entity IDs, ACS URLs
//   - Email addresses, user subjects, usernames
//   - Raw query strings, full error messages
//
// These belong in structured logs and trace spans, not in Prometheus labels.
type MonitorInterface interface {
	// ObserveHTTPRequestDuration records an HTTP request's duration
	// into the histogram. Labels: method (GET, POST, …), route (chi
	// route pattern), status (HTTP status code as string).
	ObserveHTTPRequestDuration(method, route, status string, durationSeconds float64)

	// IncrementHTTPRequestsTotal increments the total HTTP request
	// counter. Same label set as ObserveHTTPRequestDuration.
	IncrementHTTPRequestsTotal(method, route, status string)

	// IncrementBridgeOperation increments the bridge operation counter.
	// operation must be one of: oidc_code_exchange, session_create,
	// pending_request_retrieve.
	// result must be "success" or "error".
	IncrementBridgeOperation(operation, result string)
}
