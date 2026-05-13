package monitoring

// NoopMonitor is a no-op implementation of MonitorInterface.
// Used in tests and when monitoring is disabled.
type NoopMonitor struct{}

// NewNoopMonitor returns a no-op monitor.
func NewNoopMonitor() *NoopMonitor {
	return &NoopMonitor{}
}

func (*NoopMonitor) ObserveHTTPRequestDuration(_, _, _ string, _ float64) {}
func (*NoopMonitor) IncrementHTTPRequestsTotal(_, _, _ string)            {}
func (*NoopMonitor) IncrementBridgeOperation(_, _ string)                 {}
