package prometheus

import (
	"fmt"

	"github.com/canonical/identity-saml-provider/internal/monitoring"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type Monitor struct {
	service string

	responseTime           *prometheus.HistogramVec
	dependencyAvailability *prometheus.GaugeVec

	logger *zap.SugaredLogger
}

var _ monitoring.MonitorInterface = (*Monitor)(nil)

func (m *Monitor) GetService() string {
	return m.service
}

func (m *Monitor) SetResponseTimeMetric(tags map[string]string, value float64) error {
	if m.responseTime == nil {
		return fmt.Errorf("metric not instantiated")
	}

	m.responseTime.With(tags).Observe(value)
	return nil
}

func (m *Monitor) SetDependencyAvailability(tags map[string]string, value float64) error {
	if m.dependencyAvailability == nil {
		return fmt.Errorf("metric not instantiated")
	}

	m.dependencyAvailability.With(tags).Set(value)
	return nil
}

func (m *Monitor) registerHistograms() {
	histograms := make([]*prometheus.HistogramVec, 0)
	labels := map[string]string{"service": m.service}

	m.responseTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "http_response_time_seconds",
			Help:        "http_response_time_seconds",
			ConstLabels: labels,
		},
		[]string{"route", "status"},
	)

	histograms = append(histograms, m.responseTime)

	for _, histogram := range histograms {
		err := prometheus.Register(histogram)

		switch err.(type) {
		case nil:
			continue
		case prometheus.AlreadyRegisteredError:
			regErr := err.(prometheus.AlreadyRegisteredError)
			existingHistogram, ok := regErr.ExistingCollector.(*prometheus.HistogramVec)
			if !ok {
				m.logger.Errorw("existing collector is not a histogram vec", "metric", histogram)
				return
			}

			m.responseTime = existingHistogram
			m.logger.Debugw("metric already registered, reusing existing collector", "metric", histogram)
			return
		default:
			m.logger.Errorw("metric could not be registered", "metric", histogram, "error", err)
		}
	}
}

func (m *Monitor) registerGauges() {
	gauges := make([]*prometheus.GaugeVec, 0)
	labels := map[string]string{"service": m.service}

	m.dependencyAvailability = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "dependency_available",
			Help:        "dependency_available",
			ConstLabels: labels,
		},
		[]string{"component"},
	)

	gauges = append(gauges, m.dependencyAvailability)

	for _, gauge := range gauges {
		err := prometheus.Register(gauge)

		switch err.(type) {
		case nil:
			continue
		case prometheus.AlreadyRegisteredError:
			regErr := err.(prometheus.AlreadyRegisteredError)
			existingGauge, ok := regErr.ExistingCollector.(*prometheus.GaugeVec)
			if !ok {
				m.logger.Errorw("existing collector is not a gauge vec", "metric", gauge)
				return
			}

			m.dependencyAvailability = existingGauge
			m.logger.Debugw("metric already registered, reusing existing collector", "metric", gauge)
			return
		default:
			m.logger.Errorw("metric could not be registered", "metric", gauge, "error", err)
		}
	}
}

func NewMonitor(service string, logger *zap.SugaredLogger) *Monitor {
	m := new(Monitor)
	m.service = service
	m.logger = logger
	m.registerHistograms()
	m.registerGauges()
	return m
}
