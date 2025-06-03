// Copyright © 2020 Banzai Cloud
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

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	slogmulti "github.com/samber/slog-multi"
	whhttp "github.com/slok/kubewebhook/v2/pkg/http"
	whmetrics "github.com/slok/kubewebhook/v2/pkg/metrics/prometheus"
	whwebhook "github.com/slok/kubewebhook/v2/pkg/webhook"
	"github.com/slok/kubewebhook/v2/pkg/webhook/mutating"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	kubernetesConfig "sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/bank-vaults/vault-secrets-webhook/pkg/webhook"
)

func init() {
	webhook.SetConfigDefaults()
}

func newK8SClient() (kubernetes.Interface, error) {
	kubeConfig, err := kubernetesConfig.GetConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(kubeConfig)
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func handlerFor(config mutating.WebhookConfig, recorder whwebhook.MetricsRecorder) http.Handler {
	wh, err := mutating.NewWebhook(config)
	if err != nil {
		panic("error creating webhook: " + err.Error())
	}

	wh = whwebhook.NewMeasuredWebhook(recorder, wh)

	return whhttp.MustHandlerFor(whhttp.HandlerConfig{Webhook: wh, Logger: config.Logger})
}

func newHTTPServer(tlsCertFile string, tlsPrivateKeyFile string, listenAddress string, mux *http.ServeMux) *http.Server {
	reloader, err := NewCertificateReloader(tlsCertFile, tlsPrivateKeyFile)
	if err != nil {
		panic("error loading tls certificate: " + err.Error())
	}
	srv := &http.Server{
		Addr:    listenAddress,
		Handler: mux,
		TLSConfig: &tls.Config{
			GetCertificate: reloader.GetCertificateFunc(),
		},
	}
	return srv
}

func main() {
	var logger *slog.Logger
	{
		var level slog.Level

		err := level.UnmarshalText([]byte(viper.GetString("log_level")))
		if err != nil { // Silently fall back to info level
			level = slog.LevelInfo
		}

		levelFilter := func(levels ...slog.Level) func(ctx context.Context, r slog.Record) bool {
			return func(_ context.Context, r slog.Record) bool {
				return slices.Contains(levels, r.Level)
			}
		}

		router := slogmulti.Router()

		if viper.GetBool("enable_json_log") {
			// Send logs with level higher than warning to stderr
			router = router.Add(
				slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}),
				levelFilter(slog.LevelWarn, slog.LevelError),
			)

			// Send info and debug logs to stdout
			router = router.Add(
				slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}),
				levelFilter(slog.LevelDebug, slog.LevelInfo),
			)
		} else {
			// Send logs with level higher than warning to stderr
			router = router.Add(
				slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}),
				levelFilter(slog.LevelWarn, slog.LevelError),
			)

			// Send info and debug logs to stdout
			router = router.Add(
				slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}),
				levelFilter(slog.LevelDebug, slog.LevelInfo),
			)
		}

		// TODO: add level filter handler
		logger = slog.New(router.Handler())
		logger = logger.With(slog.String("app", "vault-secrets-webhook"))

		slog.SetDefault(logger)
	}

	k8sClient, err := newK8SClient()
	if err != nil {
		logger.Error(fmt.Errorf("error creating k8s client: %w", err).Error())
		os.Exit(1)
	}

	mutatingWebhook, err := webhook.NewMutatingWebhook(logger, k8sClient)
	if err != nil {
		logger.Error(fmt.Errorf("error creating mutating webhook: %w", err).Error())
		os.Exit(1)
	}

	whLogger := webhook.NewWhLogger(logger)

	mutator := webhook.ErrorLoggerMutator(mutatingWebhook.VaultSecretsMutator, whLogger)

	promRegistry := prometheus.NewRegistry()
	webhook.RegisterMetrics(promRegistry)
	metricsRecorder, err := whmetrics.NewRecorder(whmetrics.RecorderConfig{Registry: promRegistry})
	if err != nil {
		logger.Error(fmt.Errorf("error creating metrics recorder: %w", err).Error())
		os.Exit(1)
	}

	promHandler := promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{})
	podHandler := handlerFor(mutating.WebhookConfig{ID: "vault-secrets-pods", Obj: &corev1.Pod{}, Logger: whLogger, Mutator: mutator}, metricsRecorder)
	secretHandler := handlerFor(mutating.WebhookConfig{ID: "vault-secrets-secret", Obj: &corev1.Secret{}, Logger: whLogger, Mutator: mutator}, metricsRecorder)
	configMapHandler := handlerFor(mutating.WebhookConfig{ID: "vault-secrets-configmap", Obj: &corev1.ConfigMap{}, Logger: whLogger, Mutator: mutator}, metricsRecorder)
	objectHandler := handlerFor(mutating.WebhookConfig{ID: "vault-secrets-object", Obj: &unstructured.Unstructured{}, Logger: whLogger, Mutator: mutator}, metricsRecorder)

	mux := http.NewServeMux()
	mux.Handle("/pods", podHandler)
	mux.Handle("/secrets", secretHandler)
	mux.Handle("/configmaps", configMapHandler)
	mux.Handle("/objects", objectHandler)
	mux.Handle("/healthz", http.HandlerFunc(healthzHandler))

	telemetryAddress := viper.GetString("telemetry_listen_address")
	listenAddress := viper.GetString("listen_address")
	tlsCertFile := viper.GetString("tls_cert_file")
	tlsPrivateKeyFile := viper.GetString("tls_private_key_file")

	if len(telemetryAddress) > 0 {
		// Serving metrics without TLS on separated address
		go mutatingWebhook.ServeMetrics(telemetryAddress, promHandler)
	} else {
		mux.Handle("/metrics", promHandler)
	}

	if tlsCertFile == "" && tlsPrivateKeyFile == "" {
		logger.Info(fmt.Sprintf("Listening on http://%s", listenAddress))
		err = http.ListenAndServe(listenAddress, mux)
	} else {
		srv := newHTTPServer(tlsCertFile, tlsPrivateKeyFile, listenAddress, mux)
		logger.Info(fmt.Sprintf("Listening on https://%s", listenAddress))
		err = srv.ListenAndServeTLS("", "")
	}

	if err != nil {
		logger.Error(fmt.Errorf("error serving webhook: %w", err).Error())
		os.Exit(1)
	}
}
