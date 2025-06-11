// Copyright © 2025 Bank-Vaults Maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webhook

import (
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	vaultRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "vault",
			Subsystem: "client",
			Name:      "request_duration_seconds",
			Help:      "Duration of Vault client requests in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		nil,
	)
	vaultRequestSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "vault",
			Subsystem: "client",
			Name:      "request_size_bytes",
			Help:      "Size of Vault client requests in bytes.",
			Buckets:   prometheus.DefBuckets,
		},
		nil,
	)
	vaultInFlightRequestsGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "vault",
			Subsystem: "client",
			Name:      "in_flight_requests",
			Help:      "Gauge of Vault in-flight client requests.",
		},
	)
	vaultRequestsCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "vault",
			Subsystem: "client",
			Name:      "requests_total",
			Help:      "Count of Vault client requests.",
		}, []string{"code", "method"},
	)
	vaultRequestsErrorsCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "vault",
			Subsystem: "client",
			Name:      "requests_errors_total",
			Help:      "Count of Vault client request errors.",
		},
		[]string{"reason"},
	)
	vaultAuthAttemptsCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "vault",
			Subsystem: "client",
			Name:      "auth_attempts_total",
			Help:      "Count of Vault client auth attempts.",
		},
		nil,
	)
	vaultAuthAttemptsErrorsCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "vault",
			Subsystem: "client",
			Name:      "auth_attempts_errors_total",
			Help:      "Count of Vault client auth attempts errors.",
		},
		[]string{"reason"},
	)
)

// RegisterMetrics registers the Vault client metrics with Prometheus
func RegisterMetrics(registry prometheus.Registerer) {
	registry.MustRegister(vaultRequestDuration)
	registry.MustRegister(vaultRequestSize)
	registry.MustRegister(vaultInFlightRequestsGauge)
	registry.MustRegister(vaultRequestsCount)
	registry.MustRegister(vaultRequestsErrorsCount)
	registry.MustRegister(vaultAuthAttemptsCount)
	registry.MustRegister(vaultAuthAttemptsErrorsCount)
}

// InstrumentErrorsAndSizeRoundTripper instruments RoundTripper to track request errors and size
func InstrumentErrorsAndSizeRoundTripper(errCounter *prometheus.CounterVec, size *prometheus.HistogramVec, next http.RoundTripper) promhttp.RoundTripperFunc {
	return func(req *http.Request) (*http.Response, error) {
		size.WithLabelValues().Observe(float64(req.ContentLength))
		resp, err := next.RoundTrip(req)
		if err != nil {
			errCounter.WithLabelValues(mapErrorToLabel(err)).Inc()
			return nil, err
		}
		return resp, nil
	}
}

func mapErrorToLabel(err error) string {
	if strings.Contains(err.Error(), "no route to host") {
		return "no-route-to-host"
	}
	if strings.Contains(err.Error(), "i/o timeout") {
		return "io-timeout"
	}
	if strings.Contains(err.Error(), "TLS handshake timeout") {
		return "tls-handshake-timeout"
	}
	if strings.Contains(err.Error(), "TLS handshake error") {
		return "tls-handshake-error"
	}
	if strings.Contains(err.Error(), "unexpected EOF") {
		return "unexpected-eof"
	}

	return "unknown"
}
