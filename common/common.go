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

package common

import (
	"strings"
)

const (
	// Webhook annotations
	// ref: https://bank-vaults.dev/docs/mutating-webhook/annotations/
	MutateAnnotation                        = "vault.security.banzaicloud.io/mutate"
	MutateProbesAnnotation                  = "vault.security.banzaicloud.io/mutate-probes"
	TokenAuthMountAnnotation                = "vault.security.banzaicloud.io/token-auth-mount"
	VaultServiceaccountAnnotation           = "vault.security.banzaicloud.io/vault-serviceaccount"
	VaultNamespaceAnnotation                = "vault.security.banzaicloud.io/vault-namespace"
	RunAsNonRootAnnotation                  = "vault.security.banzaicloud.io/run-as-non-root"
	RunAsUserAnnotation                     = "vault.security.banzaicloud.io/run-as-user"
	RunAsGroupAnnotation                    = "vault.security.banzaicloud.io/run-as-group"
	ReadOnlyRootFsAnnotation                = "vault.security.banzaicloud.io/readonly-root-fs"
	ServiceAccountTokenVolumeNameAnnotation = "vault.security.banzaicloud.io/service-account-token-volume-name"
	PSPAllowPrivilegeEscalationAnnotation   = "vault.security.banzaicloud.io/psp-allow-privilege-escalation"
	RegistrySkipVerifyAnnotation            = "vault.security.banzaicloud.io/registry-skip-verify"
	LogLevelAnnotation                      = "vault.security.banzaicloud.io/log-level"

	// Vault annotations
	VaultAddrAnnotation                 = "vault.security.banzaicloud.io/vault-addr"
	VaultImageAnnotation                = "vault.security.banzaicloud.io/vault-image"
	VaultImagePullPolicyAnnotation      = "vault.security.banzaicloud.io/vault-image-pull-policy"
	VaultRoleAnnotation                 = "vault.security.banzaicloud.io/vault-role"
	VaultPathAnnotation                 = "vault.security.banzaicloud.io/vault-path"
	VaultSkipVerifyAnnotation           = "vault.security.banzaicloud.io/vault-skip-verify"
	VaultTLSSecretAnnotation            = "vault.security.banzaicloud.io/vault-tls-secret"
	VaultIgnoreMissingSecretsAnnotation = "vault.security.banzaicloud.io/vault-ignore-missing-secrets"
	VaultClientTimeoutAnnotation        = "vault.security.banzaicloud.io/vault-client-timeout"
	TransitKeyIDAnnotation              = "vault.security.banzaicloud.io/transit-key-id"
	TransitPathAnnotation               = "vault.security.banzaicloud.io/transit-path"
	VaultAuthMethodAnnotation           = "vault.security.banzaicloud.io/vault-auth-method"
	TransitBatchSizeAnnotation          = "vault.security.banzaicloud.io/transit-batch-size"

	// Vault agent annotations
	// ref: https://bank-vaults.dev/docs/mutating-webhook/vault-agent-templating/
	VaultAgentAnnotation                      = "vault.security.banzaicloud.io/vault-agent"
	VaultAgentConfigmapAnnotation             = "vault.security.banzaicloud.io/vault-agent-configmap"
	VaultAgentOnceAnnotation                  = "vault.security.banzaicloud.io/vault-agent-once"
	VaultAgentShareProcessNamespaceAnnotation = "vault.security.banzaicloud.io/vault-agent-share-process-namespace"
	VaultAgentCPUAnnotation                   = "vault.security.banzaicloud.io/vault-agent-cpu"
	VaultAgentCPULimitAnnotation              = "vault.security.banzaicloud.io/vault-agent-cpu-limit"
	VaultAgentCPURequestAnnotation            = "vault.security.banzaicloud.io/vault-agent-cpu-request"
	VaultAgentMemoryAnnotation                = "vault.security.banzaicloud.io/vault-agent-memory"
	VaultAgentMemoryLimitAnnotation           = "vault.security.banzaicloud.io/vault-agent-memory-limit"
	VaultAgentMemoryRequestAnnotation         = "vault.security.banzaicloud.io/vault-agent-memory-request"
	VaultConfigfilePathAnnotation             = "vault.security.banzaicloud.io/vault-configfile-path"
	VaultAgentEnvVariablesAnnotation          = "vault.security.banzaicloud.io/vault-agent-env-variables"

	// Consul template annotations
	// https://bank-vaults.dev/docs/mutating-webhook/consul-template/
	VaultConsulTemplateConfigmapAnnotation               = "vault.security.banzaicloud.io/vault-ct-configmap"
	VaultConsulTemplateImageAnnotation                   = "vault.security.banzaicloud.io/vault-ct-image"
	VaultConsulTemplateOnceAnnotation                    = "vault.security.banzaicloud.io/vault-ct-once"
	VaultConsulTemplatePullPolicyAnnotation              = "vault.security.banzaicloud.io/vault-ct-pull-policy"
	VaultConsulTemplateShareProcessNamespaceAnnotation   = "vault.security.banzaicloud.io/vault-ct-share-process-namespace"
	VaultConsulTemplateCPUAnnotation                     = "vault.security.banzaicloud.io/vault-ct-cpu"
	VaultConsulTemplateMemoryAnnotation                  = "vault.security.banzaicloud.io/vault-ct-memory"
	VaultConsuleTemplateSecretsMountPathAnnotation       = "vault.security.banzaicloud.io/vault-ct-secrets-mount-path"
	VaultConsuleTemplateInjectInInitcontainersAnnotation = "vault.security.banzaicloud.io/vault-ct-inject-in-initcontainers"

	// Vault-env/Secret-init annotations
	EnableJSONLogAnnotation = "vault.security.banzaicloud.io/enable-json-log"
	// NOTE: Change these once vault-env has been replaced with secret-init
	// SecretInitPasstroughAnnotation = "vault.security.banzaicloud.io/secret-init-passthrough"
	VaultEnvPassthroughAnnotation = "vault.security.banzaicloud.io/vault-env-passthrough"
	// SecretInitDaemonAnnotation = "vault.security.banzaicloud.io/secret-init-daemon"
	VaultEnvDaemonAnnotation = "vault.security.banzaicloud.io/vault-env-daemon"
	// SecretInitImageAnnotation = "vault.security.banzaicloud.io/secret-init-image"
	VaultEnvImageAnnotation = "vault.security.banzaicloud.io/vault-env-image"
	// SecretInitImagePullPolicyAnnotation = "vault.security.banzaicloud.io/secret-init-image-pull-policy"
	VaultEnvImagePullPolicyAnnotation = "vault.security.banzaicloud.io/vault-env-image-pull-policy"
	// VaultFromPathAnnotation = "vault.security.banzaicloud.io/vault-from-path"
	VaultEnvFromPathAnnotation = "vault.security.banzaicloud.io/vault-env-from-path"
	// SecretInitDelayAnnotation = "vault.security.banzaicloud.io/secret-init-delay"
	VaultEnvDelayAnnotation = "vault.security.banzaicloud.io/vault-env-delay"
)

func HasVaultPrefix(value string) bool {
	return strings.HasPrefix(value, "vault:") || strings.HasPrefix(value, ">>vault:")
}
