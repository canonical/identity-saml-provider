// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: Apache-2.0

package monitoring

import "go.uber.org/zap"

type NoopMonitor struct {
	service string
	logger  *zap.SugaredLogger
}

func NewNoopMonitor(service string, logger *zap.SugaredLogger) *NoopMonitor {
	m := new(NoopMonitor)
	m.service = service
	m.logger = logger
	return m
}

func (m *NoopMonitor) GetService() string {
	return m.service
}

func (m *NoopMonitor) SetResponseTimeMetric(map[string]string, float64) error {
	return nil
}

func (m *NoopMonitor) SetDependencyAvailability(map[string]string, float64) error {
	return nil
}
