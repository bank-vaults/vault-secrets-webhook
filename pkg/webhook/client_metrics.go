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
		nil,
	)
)

// RegisterMetrics registers the Vault client metrics with Prometheus
func RegisterMetrics(registry *prometheus.Registry) {
	registry.MustRegister(vaultRequestDuration)
	registry.MustRegister(vaultRequestSize)
	registry.MustRegister(vaultInFlightRequestsGauge)
	registry.MustRegister(vaultRequestsCount)
	registry.MustRegister(vaultRequestsErrorsCount)
}

// InstrumentErrorsAndSizeRoundTripper instruments RoundTripper to track request errors and size
func InstrumentErrorsAndSizeRoundTripper(counter *prometheus.CounterVec, size *prometheus.HistogramVec, next http.RoundTripper) promhttp.RoundTripperFunc {
	return func(req *http.Request) (*http.Response, error) {
		size.WithLabelValues().Observe(float64(req.ContentLength))
		resp, err := next.RoundTrip(req)
		if err != nil {
			counter.WithLabelValues().Inc()
			return nil, err
		}
		return resp, nil
	}
}
