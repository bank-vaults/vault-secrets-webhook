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
	"encoding/base64"

	"emperror.dev/errors"
	injector "github.com/bank-vaults/vault-sdk/injector/vault"
	corev1 "k8s.io/api/core/v1"

	"github.com/bank-vaults/vault-secrets-webhook/pkg/common"
)

func configMapNeedsMutation(configMap *corev1.ConfigMap) bool {
	for _, value := range configMap.Data {
		if common.HasVaultPrefix(value) || injector.HasInlineVaultDelimiters(value) {
			return true
		}
	}
	for _, value := range configMap.BinaryData {
		if common.HasVaultPrefix(string(value)) {
			return true
		}
	}

	return false
}

func (mw *MutatingWebhook) MutateConfigMap(ctx context.Context, configMap *corev1.ConfigMap, vaultConfig VaultConfig) error {
	// do an early exit and don't construct the Vault client if not needed
	if !configMapNeedsMutation(configMap) {
		return nil
	}

	vaultClient, err := mw.newVaultClient(ctx, vaultConfig)
	if err != nil {
		return errors.Wrap(err, "failed to create vault client")
	}

	defer vaultClient.Close()

	config := injector.Config{
		TransitKeyID:     vaultConfig.TransitKeyID,
		TransitPath:      vaultConfig.TransitPath,
		TransitBatchSize: vaultConfig.TransitBatchSize,
	}
	secretInjector := injector.NewSecretInjector(config, vaultClient, nil, logger)

	configMap.Data, err = secretInjector.GetDataFromVaultWithContext(ctx, configMap.Data)
	if err != nil {
		return err
	}

	for key, value := range configMap.BinaryData {
		if common.HasVaultPrefix(string(value)) {
			binaryData := map[string]string{
				key: string(value),
			}
			err := mw.mutateConfigMapBinaryData(ctx, configMap, binaryData, &secretInjector)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (mw *MutatingWebhook) mutateConfigMapBinaryData(ctx context.Context, configMap *corev1.ConfigMap, data map[string]string, secretInjector *injector.SecretInjector) error {
	mapData, err := secretInjector.GetDataFromVaultWithContext(ctx, data)
	if err != nil {
		return err
	}

	for key, value := range mapData {
		// binary data are stored in base64 inside vault
		// we need to decode base64 since k8s will encode this data too
		valueBytes, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return errors.Wrapf(err, "failed to decode ConfigMap binary data")
		}
		configMap.BinaryData[key] = valueBytes
	}

	return nil
}
