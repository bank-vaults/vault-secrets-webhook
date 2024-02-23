// Copyright © 2021 Banzai Cloud
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
	"context"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"text/template"

	"emperror.dev/errors"
	"github.com/bank-vaults/internal/injector"
	"github.com/bank-vaults/vault-sdk/vault"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/slok/kubewebhook/v2/pkg/log"
	"github.com/slok/kubewebhook/v2/pkg/model"
	"github.com/slok/kubewebhook/v2/pkg/webhook/mutating"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"

	"github.com/bank-vaults/vault-secrets-webhook/pkg/common"
)

type MutatingWebhook struct {
	k8sClient kubernetes.Interface
	namespace string
	registry  ImageRegistry
	logger    *slog.Logger
}

func (mw *MutatingWebhook) VaultSecretsMutator(ctx context.Context, ar *model.AdmissionReview, obj metav1.Object) (*mutating.MutatorResult, error) {
	vaultConfig := parseVaultConfig(obj, ar)

	if vaultConfig.Skip {
		return &mutating.MutatorResult{}, nil
	}

	// parse resulting vaultConfig.Role as potential template with fields of vaultConfig
	tmpl, err := template.New("vaultRole").Option("missingkey=error").Parse(vaultConfig.Role)
	if err != nil {
		return &mutating.MutatorResult{}, errors.Wrap(err, "error parsing vault_role")
	}
	var vRoleBuf strings.Builder
	if err = tmpl.Execute(&vRoleBuf, map[string]string{
		"authmethod":     vaultConfig.AuthMethod,
		"name":           obj.GetName(),
		"namespace":      vaultConfig.ObjectNamespace,
		"path":           vaultConfig.Path,
		"serviceaccount": vaultConfig.VaultServiceAccount,
	}); err != nil {
		return &mutating.MutatorResult{}, errors.Wrap(err, "error templating vault_role")
	}
	vaultConfig.Role = vRoleBuf.String()
	mw.logger.Debug(fmt.Sprintf("vaultConfig.Role = '%s'", vaultConfig.Role))

	switch v := obj.(type) {
	case *corev1.Pod:
		return &mutating.MutatorResult{MutatedObject: v}, mw.MutatePod(ctx, v, vaultConfig, ar.DryRun)

	case *corev1.Secret:
		return &mutating.MutatorResult{MutatedObject: v}, mw.MutateSecret(v, vaultConfig)

	case *corev1.ConfigMap:
		return &mutating.MutatorResult{MutatedObject: v}, mw.MutateConfigMap(v, vaultConfig)

	case *unstructured.Unstructured:
		return &mutating.MutatorResult{MutatedObject: v}, mw.MutateObject(v, vaultConfig)

	default:
		return &mutating.MutatorResult{}, nil
	}
}

func (mw *MutatingWebhook) getDataFromConfigmap(cmName string, ns string) (map[string]string, error) {
	configMap, err := mw.k8sClient.CoreV1().ConfigMaps(ns).Get(context.Background(), cmName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return configMap.Data, nil
}

func (mw *MutatingWebhook) getDataFromSecret(secretName string, ns string) (map[string][]byte, error) {
	secret, err := mw.k8sClient.CoreV1().Secrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return secret.Data, nil
}

func (mw *MutatingWebhook) lookForEnvFrom(envFrom []corev1.EnvFromSource, ns string) ([]corev1.EnvVar, error) {
	var envVars []corev1.EnvVar

	for _, ef := range envFrom {
		if ef.ConfigMapRef != nil {
			data, err := mw.getDataFromConfigmap(ef.ConfigMapRef.Name, ns)
			if err != nil {
				if apierrors.IsNotFound(err) || (ef.ConfigMapRef.Optional != nil && *ef.ConfigMapRef.Optional) {
					continue
				}

				return envVars, err
			}
			for key, value := range data {
				if common.HasVaultPrefix(value) || injector.HasInlineVaultDelimiters(value) {
					envFromCM := corev1.EnvVar{
						Name:  key,
						Value: value,
					}
					envVars = append(envVars, envFromCM)
				}
			}
		}
		if ef.SecretRef != nil {
			data, err := mw.getDataFromSecret(ef.SecretRef.Name, ns)
			if err != nil {
				if apierrors.IsNotFound(err) || (ef.SecretRef.Optional != nil && *ef.SecretRef.Optional) {
					continue
				}

				return envVars, err
			}
			for name, v := range data {
				value := string(v)
				if common.HasVaultPrefix(value) || injector.HasInlineVaultDelimiters(value) {
					envFromSec := corev1.EnvVar{
						Name:  name,
						Value: value,
					}
					envVars = append(envVars, envFromSec)
				}
			}
		}
	}
	return envVars, nil
}

func (mw *MutatingWebhook) lookForValueFrom(env corev1.EnvVar, ns string) (*corev1.EnvVar, error) {
	if env.ValueFrom.ConfigMapKeyRef != nil {
		data, err := mw.getDataFromConfigmap(env.ValueFrom.ConfigMapKeyRef.Name, ns)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}
		value := data[env.ValueFrom.ConfigMapKeyRef.Key]
		if common.HasVaultPrefix(value) || injector.HasInlineVaultDelimiters(value) {
			fromCM := corev1.EnvVar{
				Name:  env.Name,
				Value: value,
			}
			return &fromCM, nil
		}
	}
	if env.ValueFrom.SecretKeyRef != nil {
		data, err := mw.getDataFromSecret(env.ValueFrom.SecretKeyRef.Name, ns)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}
		value := string(data[env.ValueFrom.SecretKeyRef.Key])
		if common.HasVaultPrefix(value) || injector.HasInlineVaultDelimiters(value) {
			fromSecret := corev1.EnvVar{
				Name:  env.Name,
				Value: value,
			}
			return &fromSecret, nil
		}
	}
	return nil, nil
}

func (mw *MutatingWebhook) newVaultClient(vaultConfig VaultConfig) (*vault.Client, error) {
	clientConfig := vaultapi.DefaultConfig()
	if clientConfig.Error != nil {
		return nil, clientConfig.Error
	}

	clientConfig.Address = vaultConfig.Addr

	tlsConfig := vaultapi.TLSConfig{Insecure: vaultConfig.SkipVerify}
	err := clientConfig.ConfigureTLS(&tlsConfig)
	if err != nil {
		return nil, err
	}

	if vaultConfig.TLSSecret != "" {
		tlsSecret, err := mw.k8sClient.CoreV1().Secrets(mw.namespace).Get(
			context.Background(),
			vaultConfig.TLSSecret,
			metav1.GetOptions{},
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read Vault TLS Secret")
		}

		clientTLSConfig := clientConfig.HttpClient.Transport.(*http.Transport).TLSClientConfig

		pool := x509.NewCertPool()

		ok := pool.AppendCertsFromPEM(tlsSecret.Data["ca.crt"])
		if !ok {
			return nil, errors.Errorf("error loading Vault CA PEM from TLS Secret: %s", tlsSecret.Name)
		}

		clientTLSConfig.RootCAs = pool
	}

	if vaultConfig.VaultServiceAccount != "" {
		sa, err := mw.k8sClient.CoreV1().ServiceAccounts(vaultConfig.ObjectNamespace).Get(context.Background(), vaultConfig.VaultServiceAccount, metav1.GetOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "Failed to retrieve specified service account on namespace "+vaultConfig.ObjectNamespace)
		}

		saToken := ""
		if len(sa.Secrets) > 0 {
			secret, err := mw.k8sClient.CoreV1().Secrets(vaultConfig.ObjectNamespace).Get(context.Background(), sa.Secrets[0].Name, metav1.GetOptions{})
			if err != nil {
				return nil, errors.Wrap(err, "Failed to retrieve secret for service account "+sa.Secrets[0].Name+" in namespace "+vaultConfig.ObjectNamespace)
			}
			saToken = string(secret.Data["token"])
		}

		if saToken == "" {
			tokenTTL := int64(600) // min allowed duration is 10 mins
			tokenRequest := &authenticationv1.TokenRequest{
				Spec: authenticationv1.TokenRequestSpec{
					Audiences:         []string{"https://kubernetes.default.svc"},
					ExpirationSeconds: &tokenTTL,
				},
			}

			token, err := mw.k8sClient.CoreV1().ServiceAccounts(vaultConfig.ObjectNamespace).CreateToken(
				context.Background(),
				vaultConfig.VaultServiceAccount,
				tokenRequest,
				metav1.CreateOptions{},
			)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to create a token for the specified service account "+vaultConfig.VaultServiceAccount+" on namespace "+vaultConfig.ObjectNamespace)
			}
			saToken = token.Status.Token
		}

		return vault.NewClientFromConfig(
			clientConfig,
			vault.ClientRole(vaultConfig.Role),
			vault.ClientAuthPath(vaultConfig.Path),
			vault.NamespacedSecretAuthMethod,
			vault.ClientLogger(&clientLogger{logger: mw.logger}),
			vault.ExistingSecret(saToken),
			vault.VaultNamespace(vaultConfig.VaultNamespace),
		)
	}

	return vault.NewClientFromConfig(
		clientConfig,
		vault.ClientRole(vaultConfig.Role),
		vault.ClientAuthPath(vaultConfig.Path),
		vault.ClientAuthMethod(vaultConfig.AuthMethod),
		vault.ClientLogger(&clientLogger{logger: mw.logger}),
		vault.VaultNamespace(vaultConfig.VaultNamespace),
	)
}

func (mw *MutatingWebhook) ServeMetrics(addr string, handler http.Handler) {
	mw.logger.Info(fmt.Sprintf("Telemetry on http://%s", addr))

	mux := http.NewServeMux()
	mux.Handle("/metrics", handler)
	err := http.ListenAndServe(addr, mux)
	if err != nil {
		mw.logger.Error(fmt.Errorf("error serving telemetry: %w", err).Error())
		os.Exit(1)
	}
}

func NewMutatingWebhook(logger *slog.Logger, k8sClient kubernetes.Interface) (*MutatingWebhook, error) {
	namespace := os.Getenv("KUBERNETES_NAMESPACE") // only for kurun
	if namespace == "" {
		namespaceBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			return nil, errors.Wrap(err, "error reading k8s namespace")
		}
		namespace = string(namespaceBytes)
	}

	return &MutatingWebhook{
		k8sClient: k8sClient,
		namespace: namespace,
		registry:  NewRegistry(),
		logger:    logger,
	}, nil
}

func ErrorLoggerMutator(mutator mutating.MutatorFunc, logger log.Logger) mutating.MutatorFunc {
	return func(ctx context.Context, ar *model.AdmissionReview, obj metav1.Object) (result *mutating.MutatorResult, err error) {
		r, err := mutator(ctx, ar, obj)
		if err != nil {
			logger.WithCtxValues(ctx).WithValues(log.Kv{
				"error": err,
			}).Errorf("Admission review request failed")
		}
		return r, err
	}
}
