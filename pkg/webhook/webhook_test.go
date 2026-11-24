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
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/slok/kubewebhook/v2/pkg/model"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/bank-vaults/vault-secrets-webhook/pkg/common"
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
				return fake.NewClientset(
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
				return fake.NewClientset()
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

			client, err := mw.newVaultClient(t.Context(), tt.vaultConfig)

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

func TestParseVaultConfigRejectsObjectAddr(t *testing.T) {
	tests := []struct {
		name      string
		addr      string
		allowlist string
		wantErr   bool
	}{
		{
			name:    "PoC IMDS address from object annotation is rejected",
			addr:    "http://169.254.169.254/latest/meta-data/",
			wantErr: true,
		},
		{
			name:    "non-allowlisted external address from object annotation is rejected",
			addr:    "https://evil.attacker.com",
			wantErr: true,
		},
		{
			name:      "userinfo-bearing address from object annotation is rejected",
			addr:      "https://attacker:pw@vault.prod.svc:8200",
			allowlist: "https://vault.prod.svc:8200",
			wantErr:   true,
		},
		{
			name:      "allowlisted address from object annotation passes validation",
			addr:      "https://vault.prod.svc:8200",
			allowlist: "https://vault.prod.svc:8200",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set("vault_addr_allowlist", tt.allowlist)
			t.Cleanup(viper.Reset)

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{common.VaultAddrAnnotation: tt.addr},
				},
			}

			_, err := parseVaultConfig(pod, &model.AdmissionReview{})

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "rejected Vault address from object annotation")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestVaultTLSSecretNamespace(t *testing.T) {
	const (
		webhookNamespace = "webhook"
		objectNamespace  = "workload"
		secretName       = "vault-ca"
	)

	for _, tt := range []struct {
		name, scope, namespace, secretName string
		enabled, localSecretExists, pod    bool
		wantNamespace, wantError           string
	}{
		{
			name: "defaults to webhook namespace", namespace: objectNamespace, enabled: true,
			wantNamespace: webhookNamespace, wantError: "error loading Vault CA PEM",
		},
		{
			name: "selects object namespace", scope: "object", namespace: objectNamespace,
			enabled: true, localSecretExists: true,
			wantNamespace: objectNamespace, wantError: "error loading Vault CA PEM",
		},
		{
			name: "missing local Secret does not fall back", scope: "object",
			namespace: objectNamespace, enabled: true,
			wantNamespace: objectNamespace, wantError: "failed to read Vault TLS Secret workload/vault-ca",
		},
		{
			name: "requires administrator opt-in", scope: "object", namespace: objectNamespace,
			wantError: "require VAULT_ALLOW_OBJECT_TLS_SECRET_NAMESPACE",
		},
		{
			name: "rejects cluster scoped objects", scope: "object", enabled: true,
			wantError: "require a namespaced object",
		},
		{
			name: "rejects arbitrary scope", scope: "other", namespace: objectNamespace, enabled: true,
			wantError: "expected object",
		},
		{
			name: "rejects qualified Secret name", scope: "object", namespace: objectNamespace,
			secretName: "other/vault-ca", enabled: true, wantError: "must specify a valid Secret name",
		},
		{
			name: "Pod configuration does not require opt-in", scope: "object", namespace: objectNamespace,
			pod: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			SetConfigDefaults()
			t.Cleanup(viper.Reset)
			t.Setenv("VAULT_ALLOW_OBJECT_TLS_SECRET_NAMESPACE", strconv.FormatBool(tt.enabled))

			name := tt.secretName
			if name == "" {
				name = secretName
			}
			meta := metav1.ObjectMeta{Annotations: map[string]string{common.VaultTLSSecretAnnotation: name}}
			if tt.scope != "" {
				meta.Annotations[common.VaultTLSSecretScopeAnnotation] = tt.scope
			}
			var obj metav1.Object = &corev1.Secret{ObjectMeta: meta}
			if tt.pod {
				obj = &corev1.Pod{ObjectMeta: meta}
			}

			config, err := parseVaultConfig(obj, &model.AdmissionReview{Namespace: tt.namespace})
			if tt.pod {
				require.NoError(t, err)
				assert.Equal(t, secretName, config.TLSSecret)
				assert.Empty(t, config.TLSSecretNamespace)
				return
			}
			if tt.wantNamespace == "" {
				require.ErrorContains(t, err, tt.wantError)
				return
			}
			require.NoError(t, err)

			// Invalid CA data stops client setup after the lookup, before Vault auth.
			newCASecret := func(namespace string) *corev1.Secret {
				return &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace},
					Data:       map[string][]byte{"ca.crt": []byte("invalid PEM")},
				}
			}
			secrets := []runtime.Object{newCASecret(webhookNamespace)}
			if tt.localSecretExists {
				secrets = append(secrets, newCASecret(objectNamespace))
			}
			k8sClient := fake.NewClientset(secrets...)
			mw := &MutatingWebhook{k8sClient: k8sClient, namespace: webhookNamespace}

			_, err = mw.newVaultClient(t.Context(), config)
			require.ErrorContains(t, err, tt.wantError)
			actions := k8sClient.Actions()
			require.Len(t, actions, 1)
			assert.True(t, actions[0].Matches("get", "secrets"))
			assert.Equal(t, tt.wantNamespace, actions[0].GetNamespace())
		})
	}
}
