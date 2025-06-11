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
	"net/http/httptest"
	"os"
	"testing"

	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewVaultClientMetrics(t *testing.T) {
	prometheus.DefaultRegisterer.MustRegister(vaultAuthAttemptsCount)
	prometheus.DefaultRegisterer.MustRegister(vaultAuthAttemptsErrorsCount)
	logger := slog.New(slog.DiscardHandler)

	// Skip trying to read the namespace from the file
	require.NoError(t, os.Setenv("KUBERNETES_NAMESPACE", "test-namespace"))

	tests := []struct {
		name          string
		vaultConfig   VaultConfig
		expectedError bool
		setupK8s      func(t *testing.T) *fake.Clientset
		handler       http.HandlerFunc
	}{
		{
			name: "successful vault client creation with service account token",
			vaultConfig: VaultConfig{
				Addr:                "https://vault.example.com",
				SkipVerify:          true,
				Role:                "test-role",
				Path:                "kubernetes",
				VaultServiceAccount: "test-sa",
				ObjectNamespace:     "test-namespace",
			},
			setupK8s: func(t *testing.T) *fake.Clientset {
				return fake.NewSimpleClientset(
					&corev1.ServiceAccount{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-sa",
							Namespace: "test-namespace",
						},
						Secrets: []corev1.ObjectReference{
							{
								Name: "test-sa-token-xyz",
							},
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-sa-token-xyz",
							Namespace: "test-namespace",
						},
						Data: map[string][]byte{
							"token": []byte("test-token"),
						},
						Type: corev1.SecretTypeServiceAccountToken,
					},
				)
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"auth": {"client_token": "test-token"}}`))
				require.NoError(t, err)
			},
			expectedError: false,
		},
		{
			name: "error when service account not found",
			vaultConfig: VaultConfig{
				Addr:                "https://127.0.0.1:8082",
				SkipVerify:          true,
				Role:                "test-role",
				Path:                "kubernetes",
				VaultServiceAccount: "non-existent-sa",
				ObjectNamespace:     "test-namespace",
			},
			setupK8s: func(t *testing.T) *fake.Clientset {
				return fake.NewSimpleClientset()
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vaultAuthAttemptsCount.Reset()
			vaultAuthAttemptsErrorsCount.Reset()

			k8sClient := tt.setupK8s(t)
			mw, err := NewMutatingWebhook(logger, k8sClient)
			require.NoError(t, err)

			if tt.handler != nil {
				server := httptest.NewServer(tt.handler)
				tt.vaultConfig.Addr = server.URL
				defer server.Close()
			}

			client, err := mw.newVaultClient(tt.vaultConfig)

			assert.Equal(t, float64(1), testutil.ToFloat64(vaultAuthAttemptsCount.WithLabelValues()), "vaultAuthAttemptsCount should be incremented")
			if tt.expectedError {
				assert.Equal(t, float64(1), testutil.ToFloat64(vaultAuthAttemptsErrorsCount.WithLabelValues("kubernetes_error")), "vaultAuthAttemptsErrorsCount should be incremented on error")
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.Equal(t, float64(0), testutil.ToFloat64(vaultAuthAttemptsErrorsCount.WithLabelValues("kubernetes_error")), "vaultAuthAttemptsErrorsCount should not be incremented on success")
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}
