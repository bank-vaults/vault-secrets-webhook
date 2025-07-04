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
	"fmt"
	"strings"

	"emperror.dev/errors"
	injector "github.com/bank-vaults/vault-sdk/injector/vault"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/bank-vaults/vault-secrets-webhook/pkg/common"
)

type element interface {
	Set(v interface{})
	Get() interface{}
}

type iterator <-chan element

type mapElement struct {
	m map[string]interface{}
	k string
}

func (e *mapElement) Set(v interface{}) {
	e.m[e.k] = v
}

func (e *mapElement) Get() interface{} {
	return e.m[e.k]
}

type sliceElement struct {
	s []interface{}
	i int
}

func (e *sliceElement) Set(v interface{}) {
	e.s[e.i] = v
}

func (e *sliceElement) Get() interface{} {
	return e.s[e.i]
}

func mapIterator(m map[string]interface{}) iterator {
	c := make(chan element, len(m))
	for k := range m {
		c <- &mapElement{k: k, m: m}
	}
	close(c)
	return c
}

func sliceIterator(s []interface{}) iterator {
	c := make(chan element, len(s))
	for i := range s {
		c <- &sliceElement{i: i, s: s}
	}
	close(c)
	return c
}

func traverseObject(ctx context.Context, o interface{}, secretInjector *injector.SecretInjector) error {
	var iterator iterator

	switch value := o.(type) {
	case map[string]interface{}:
		iterator = mapIterator(value)
	case []interface{}:
		iterator = sliceIterator(value)
	default:
		return nil
	}

	for e := range iterator {
		switch s := e.Get().(type) {
		case string:
			if common.HasVaultPrefix(s) {
				dataFromVault, err := secretInjector.GetDataFromVaultWithContext(ctx, map[string]string{"data": s})
				if err != nil {
					return err
				}

				e.Set(dataFromVault["data"])
			} else if injector.HasInlineVaultDelimiters(s) {
				dataFromVault := s
				for _, vaultSecretReference := range injector.FindInlineVaultDelimiters(s) {
					mapData, err := secretInjector.GetDataFromVaultWithContext(ctx, map[string]string{"data": vaultSecretReference[1]})
					if err != nil {
						return err
					}
					dataFromVault = strings.ReplaceAll(dataFromVault, vaultSecretReference[0], mapData["data"])
				}
				e.Set(dataFromVault)
			}
		case map[string]interface{}, []interface{}:
			err := traverseObject(ctx, e.Get(), secretInjector)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (mw *MutatingWebhook) MutateObject(ctx context.Context, object *unstructured.Unstructured, vaultConfig VaultConfig) error {
	mw.logger.Debug(fmt.Sprintf("mutating object: %s.%s", object.GetNamespace(), object.GetName()))

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

	return traverseObject(ctx, object.Object, &secretInjector)
}
