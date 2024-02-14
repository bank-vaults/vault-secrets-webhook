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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"emperror.dev/errors"
	"github.com/bank-vaults/internal/injector"
	corev1 "k8s.io/api/core/v1"
)

type dockerCredentials struct {
	Auths map[string]dockerAuthConfig `json:"auths"`
}

// dockerAuthConfig contains authorization information for connecting to a Registry
type dockerAuthConfig struct {
	Username string      `json:"username,omitempty"`
	Password string      `json:"password,omitempty"`
	Auth     interface{} `json:"auth,omitempty"`

	// Email is an optional value associated with the username.
	// This field is deprecated and will be removed in a later
	// version of docker.
	Email string `json:"email,omitempty"`

	ServerAddress string `json:"serveraddress,omitempty"`

	// IdentityToken is used to authenticate the user and get
	// an access token for the registry.
	IdentityToken string `json:"identitytoken,omitempty"`

	// RegistryToken is a bearer token to be sent to a registry
	RegistryToken string `json:"registrytoken,omitempty"`
}

func secretNeedsMutation(secret *corev1.Secret) (bool, error) {
	for key, value := range secret.Data {
		if key == corev1.DockerConfigJsonKey {
			var dc dockerCredentials
			err := json.Unmarshal(value, &dc)
			if err != nil {
				return false, errors.Wrap(err, "unmarshal dockerconfig json failed")
			}

			for _, creds := range dc.Auths {
				switch creds.Auth.(type) {
				case string:
					authBytes, err := base64.StdEncoding.DecodeString(creds.Auth.(string))
					if err != nil {
						return false, errors.Wrap(err, "auth base64 decoding failed")
					}

					auth := string(authBytes)
					if hasVaultPrefix(auth) {
						return true, nil
					}

				case map[string]interface{}:
					// get sub-keys from the auth field
					authMap, ok := creds.Auth.(map[string]interface{})
					if !ok {
						return false, errors.New("invalid auth type")
					}

					// check if any of the sub-keys have a vault prefix
					for _, v := range authMap {
						if hasVaultPrefix(v.(string)) {
							return true, nil
						}
					}
					return false, nil

				default:
					return false, errors.New("invalid auth type")
				}
			}
		} else if hasVaultPrefix(string(value)) {
			return true, nil
		} else if injector.HasInlineVaultDelimiters(string(value)) {
			return true, nil
		}
	}
	return false, nil
}

func (mw *MutatingWebhook) MutateSecret(secret *corev1.Secret, vaultConfig VaultConfig) error {
	// do an early exit and don't construct the Vault client if not needed
	requiredToMutate, err := secretNeedsMutation(secret)
	if err != nil {
		return errors.Wrap(err, "failed to check if secret needs to be mutated")
	}

	if !requiredToMutate {
		return nil
	}

	vaultClient, err := mw.newVaultClient(vaultConfig)
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

	if value, ok := secret.Data[corev1.DockerConfigJsonKey]; ok {
		var dc dockerCredentials
		err := json.Unmarshal(value, &dc)
		if err != nil {
			return errors.Wrap(err, "unmarshal dockerconfig json failed")
		}
		err = mw.mutateDockerCreds(secret, &dc, &secretInjector)
		if err != nil {
			return errors.Wrap(err, "mutate dockerconfig json failed")
		}
	}

	err = mw.mutateSecretData(secret, &secretInjector)
	if err != nil {
		return errors.Wrap(err, "mutate generic secret failed")
	}

	return nil
}

func (mw *MutatingWebhook) mutateDockerCreds(secret *corev1.Secret, dc *dockerCredentials, secretInjector *injector.SecretInjector) error {
	assembled := dockerCredentials{Auths: map[string]dockerAuthConfig{}}

	for key, creds := range dc.Auths {
		authBytes, err := base64.StdEncoding.DecodeString(creds.Auth.(string))
		if err != nil {
			return errors.Wrap(err, "auth base64 decoding failed")
		}

		auth := string(authBytes)
		if hasVaultPrefix(auth) {
			username, password, err := handleAuthString(auth)
			if err != nil {
				return errors.Wrap(err, "invalid auth string")
			}

			credentialData := map[string]string{
				"username": username,
				"password": password,
			}

			dcCreds, err := secretInjector.GetDataFromVault(credentialData)
			if err != nil {
				return err
			}
			auth = fmt.Sprintf("%s:%s", dcCreds["username"], dcCreds["password"])
			dockerAuth := dockerAuthConfig{
				Auth: base64.StdEncoding.EncodeToString([]byte(auth)),
			}
			if creds.Username != "" && creds.Password != "" {
				dockerAuth.Username = dcCreds["username"]
				dockerAuth.Password = dcCreds["password"]
			}
			assembled.Auths[key] = dockerAuth
		}
	}

	marshaled, err := json.Marshal(assembled)
	if err != nil {
		return errors.Wrap(err, "marshaling dockerconfig failed")
	}

	secret.Data[corev1.DockerConfigJsonKey] = marshaled

	return nil
}

func (mw *MutatingWebhook) mutateSecretData(secret *corev1.Secret, secretInjector *injector.SecretInjector) error {
	convertedData := make(map[string]string, len(secret.Data))

	for k := range secret.Data {
		convertedData[k] = string(secret.Data[k])
	}

	convertedData, err := secretInjector.GetDataFromVault(convertedData)
	if err != nil {
		return err
	}

	for k := range secret.Data {
		secret.Data[k] = []byte(convertedData[k])
	}

	return nil
}

func handleAuthString(auth string) (string, string, error) {
	// if the auth string is formatted as "username:usr:password:pass",
	// split the string into username and password
	split := strings.Split(auth, ":")
	if len(split) == 4 {
		username := fmt.Sprintf("%s:%s", split[0], split[1])
		password := fmt.Sprintf("%s:%s", split[2], split[3])

		return username, password, nil
	}

	// if the auth string is a JSON key,
	// don't split and use it as is
	if isJSONKey(auth) {
		return auth, "", nil
	}

	// if none of the above, the auth string can still be a valid vault path
	if validVaultPath(auth) {
		return auth, "", nil
	}

	return "", "", errors.New("invalid auth string")
}

func isJSONKey(auth string) bool {
	var authMap map[string]interface{}
	err := json.Unmarshal([]byte(auth), &authMap)
	if err != nil {
		return false
	}

	// if there are sub-keys present under the auth key
	// assume a JSON key
	if len(authMap) > 0 {
		return true
	}

	return false
}

// hasVaultPrefix checks if the given string is a valid vault path
func validVaultPath(auth string) bool {
	re := regexp.MustCompile(`^(vault:secret)(\/\w+)+(#.+)`)
	match := re.MatchString(auth)

	return match
}
