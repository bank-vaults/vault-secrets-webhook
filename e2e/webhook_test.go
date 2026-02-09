// Copyright © 2023 Bank-Vaults Maintainers
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

//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestSecretValueInjection(t *testing.T) {
	secret := applyResource(features.New("secret"), "secret.yaml").
		Assess("object created", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			secrets := &v1.SecretList{
				Items: []v1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-secret", Namespace: cfg.Namespace()},
					},
				},
			}

			// wait for the secret to become available
			err := wait.For(conditions.New(cfg.Client().Resources()).ResourcesFound(secrets), wait.WithTimeout(1*time.Minute))
			require.NoError(t, err)

			return ctx
		}).
		Assess("secret values are injected", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var secret v1.Secret

			err := cfg.Client().Resources(cfg.Namespace()).Get(ctx, "test-secret", cfg.Namespace(), &secret)
			require.NoError(t, err)

			type v1 struct {
				Username string `json:"username"`
				Password string `json:"password"`
				Auth     string `json:"auth"`
			}

			type auths struct {
				V1 v1 `json:"https://index.docker.io/v1/"`
			}

			type dockerconfig struct {
				Auths auths `json:"auths"`
			}

			var dockerconfigjson dockerconfig

			err = json.Unmarshal(secret.Data[".dockerconfigjson"], &dockerconfigjson)
			require.NoError(t, err)

			assert.Equal(t, "dockerrepouser", dockerconfigjson.Auths.V1.Username)
			assert.Equal(t, "dockerrepopassword", dockerconfigjson.Auths.V1.Password)
			assert.Equal(t, "Inline: secretId AWS_ACCESS_KEY_ID", string(secret.Data["inline"]))

			return ctx
		}).
		Feature()

	configMap := applyResource(features.New("configmap"), "configmap.yaml").
		Assess("object created", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			configMaps := &v1.ConfigMapList{
				Items: []v1.ConfigMap{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-configmap", Namespace: cfg.Namespace()},
					},
				},
			}

			// wait for the secret to become available
			err := wait.For(conditions.New(cfg.Client().Resources()).ResourcesFound(configMaps), wait.WithTimeout(1*time.Minute))
			require.NoError(t, err)

			return ctx
		}).
		Assess("secret values are injected", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var configMap v1.ConfigMap

			err := cfg.Client().Resources(cfg.Namespace()).Get(ctx, "test-configmap", cfg.Namespace(), &configMap)
			require.NoError(t, err)

			assert.Equal(t, "secretId", string(configMap.Data["aws-access-key-id"]))
			assert.Equal(t, "AWS key in base64: c2VjcmV0SWQ=", string(configMap.Data["aws-access-key-id-formatted"]))
			assert.Equal(t, "AWS_ACCESS_KEY_ID: secretId AWS_SECRET_ACCESS_KEY: s3cr3t", string(configMap.Data["aws-access-key-id-inline"]))
			assert.Equal(t, "secretId", base64.StdEncoding.EncodeToString(configMap.BinaryData["aws-access-key-id-binary"]))

			return ctx
		}).
		Feature()

	testenv.Test(t, secret, configMap)
}

func TestPodMutation(t *testing.T) {
	deployment := applyResource(features.New("deployment"), "deployment.yaml").
		Assess("available", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "test-deployment", Namespace: cfg.Namespace()},
			}

			// wait for the deployment to become available
			err := wait.For(conditions.New(cfg.Client().Resources()).DeploymentConditionMatch(deployment, appsv1.DeploymentAvailable, v1.ConditionTrue), wait.WithTimeout(2*time.Minute))
			require.NoError(t, err)

			return ctx
		}).
		Assess("security context defaults are correct", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r := cfg.Client().Resources()

			pods := &v1.PodList{}

			err := r.List(ctx, pods, resources.WithLabelSelector("app.kubernetes.io/name=test-deployment"))
			require.NoError(t, err)

			if pods == nil || len(pods.Items) == 0 {
				t.Fatal("no pods found")
			}

			securityContext := pods.Items[0].Spec.InitContainers[0].SecurityContext

			assert.Nil(t, securityContext.RunAsNonRoot)
			assert.Nil(t, securityContext.RunAsUser)
			assert.Nil(t, securityContext.RunAsGroup)

			return ctx
		}).
		Feature()

	deploymentSeccontext := applyResource(features.New("deployment-seccontext"), "deployment-seccontext.yaml").
		Assess("available", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "test-deployment-seccontext", Namespace: cfg.Namespace()},
			}

			// wait for the deployment to become available
			err := wait.For(conditions.New(cfg.Client().Resources()).DeploymentConditionMatch(deployment, appsv1.DeploymentAvailable, v1.ConditionTrue), wait.WithTimeout(2*time.Minute))
			require.NoError(t, err)

			return ctx
		}).
		Feature()

	deploymentTemplating := applyResource(features.New("deployment-template"), "deployment-template.yaml").
		Assess("available", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r := cfg.Client().Resources()

			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "test-deployment-template", Namespace: cfg.Namespace()},
			}

			// wait for the deployment to become available
			err := wait.For(conditions.New(r).DeploymentConditionMatch(deployment, appsv1.DeploymentAvailable, v1.ConditionTrue), wait.WithTimeout(2*time.Minute))
			require.NoError(t, err)

			return ctx
		}).
		Assess("config template", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r := cfg.Client().Resources()

			pods := &v1.PodList{}

			err := r.List(ctx, pods, resources.WithLabelSelector("app.kubernetes.io/name=test-deployment-template"))
			require.NoError(t, err)

			if pods == nil || len(pods.Items) == 0 {
				t.Fatal("no pods found")
			}

			// wait for the container to become available
			err = wait.For(conditions.New(r).ContainersReady(&pods.Items[0]), wait.WithTimeout(2*time.Minute))
			require.NoError(t, err)

			var stdout, stderr bytes.Buffer
			podName := pods.Items[0].Name
			command := []string{"cat", "/vault/secrets/config.yaml"}

			if err := r.ExecInPod(context.TODO(), cfg.Namespace(), podName, "alpine", command, &stdout, &stderr); err != nil {
				t.Log(stderr.String())
				t.Fatal(err)
			}

			assert.Equal(t, "\n    {\n      \"id\": \"secretId\",\n      \"key\": \"s3cr3t\"\n    }\n    \n  ", stdout.String())

			return ctx
		}).
		Feature()

	deploymentInitSeccontext := applyResource(features.New("deployment-init-seccontext"), "deployment-init-seccontext.yaml").
		Assess("available", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "test-deployment-init-seccontext", Namespace: cfg.Namespace()},
			}

			// wait for the deployment to become available
			err := wait.For(conditions.New(cfg.Client().Resources()).DeploymentConditionMatch(deployment, appsv1.DeploymentAvailable, v1.ConditionTrue), wait.WithTimeout(2*time.Minute))
			require.NoError(t, err)

			return ctx
		}).
		Assess("security context is correct", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r := cfg.Client().Resources()

			pods := &v1.PodList{}

			err := r.List(ctx, pods, resources.WithLabelSelector("app.kubernetes.io/name=test-deployment-init-seccontext"))
			require.NoError(t, err)

			if pods == nil || len(pods.Items) == 0 {
				t.Fatal("no pods found")
			}

			// wait for the container to become available
			err = wait.For(conditions.New(r).ContainersReady(&pods.Items[0]), wait.WithTimeout(2*time.Minute))
			require.NoError(t, err)

			securityContext := pods.Items[0].Spec.InitContainers[0].SecurityContext

			require.NotNil(t, securityContext.RunAsNonRoot)
			assert.Equal(t, true, *securityContext.RunAsNonRoot)
			require.NotNil(t, securityContext.RunAsUser)
			assert.Equal(t, int64(1000), *securityContext.RunAsUser)
			require.NotNil(t, securityContext.RunAsGroup)
			assert.Equal(t, int64(1000), *securityContext.RunAsGroup)

			return ctx
		}).
		Feature()

	deploymentMutateContainers := applyResource(features.New("deployment-mutate-containers"), "deployment-mutate-containers.yaml").
		Assess("available", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "test-deployment-mutate-containers", Namespace: cfg.Namespace()},
			}

			// wait for the deployment to become available
			err := wait.For(conditions.New(cfg.Client().Resources()).DeploymentConditionMatch(deployment, appsv1.DeploymentAvailable, v1.ConditionTrue), wait.WithTimeout(2*time.Minute))
			require.NoError(t, err)

			return ctx
		}).
		Assess("only specified containers are mutated", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r := cfg.Client().Resources()

			pods := &v1.PodList{}

			err := r.List(ctx, pods, resources.WithLabelSelector("app.kubernetes.io/name=test-deployment-mutate-containers"))
			require.NoError(t, err)

			if pods == nil || len(pods.Items) == 0 {
				t.Fatal("no pods found")
			}

			// wait for the container to become available
			err = wait.For(conditions.New(r).ContainersReady(&pods.Items[0]), wait.WithTimeout(2*time.Minute))
			require.NoError(t, err)

			pod := pods.Items[0]

			// Check that alpine container was mutated (should have /vault/vault-env in command)
			alpineContainer := findContainerByName(pod.Spec.Containers, "alpine")
			require.NotNil(t, alpineContainer, "alpine container not found")
			require.Greater(t, len(alpineContainer.Command), 0, "alpine container should have command")
			assert.Equal(t, "/vault/vault-env", alpineContainer.Command[0], "alpine container should be mutated")
			// Check for vault-env volumeMount
			assert.True(t, hasVolumeMount(alpineContainer.VolumeMounts, "vault-env"), "alpine container should have vault-env volumeMount")

			// Check that redis container was mutated (should have /vault/vault-env in command)
			redisContainer := findContainerByName(pod.Spec.Containers, "redis")
			require.NotNil(t, redisContainer, "redis container not found")
			require.Greater(t, len(redisContainer.Command), 0, "redis container should have command")
			assert.Equal(t, "/vault/vault-env", redisContainer.Command[0], "redis container should be mutated")
			// Check for vault-env volumeMount
			assert.True(t, hasVolumeMount(redisContainer.VolumeMounts, "vault-env"), "redis container should have vault-env volumeMount")

			// Check that nginx container was NOT mutated (should not have /vault/vault-env in command)
			nginxContainer := findContainerByName(pod.Spec.Containers, "nginx")
			require.NotNil(t, nginxContainer, "nginx container not found")
			if len(nginxContainer.Command) > 0 {
				assert.NotEqual(t, "/vault/vault-env", nginxContainer.Command[0], "nginx container should NOT be mutated")
			}
			// Check absence of vault-env volumeMount
			assert.False(t, hasVolumeMount(nginxContainer.VolumeMounts, "vault-env"), "nginx container should NOT have vault-env volumeMount")

			// Check that init-ubuntu container was NOT mutated (not specified in annotation)
			initContainer := findContainerByName(pod.Spec.InitContainers, "init-ubuntu")
			require.NotNil(t, initContainer, "init-ubuntu container not found")
			if len(initContainer.Command) > 0 {
				assert.NotEqual(t, "/vault/vault-env", initContainer.Command[0], "init-ubuntu container should NOT be mutated")
			}
			// Check absence of vault-env volumeMount
			assert.False(t, hasVolumeMount(initContainer.VolumeMounts, "vault-env"), "init-ubuntu container should NOT have vault-env volumeMount")

			return ctx
		}).
		Feature()

	testenv.Test(t, deployment, deploymentSeccontext, deploymentTemplating, deploymentInitSeccontext, deploymentMutateContainers)
}

func findContainerByName(containers []v1.Container, name string) *v1.Container {
	for i := range containers {
		if containers[i].Name == name {
			return &containers[i]
		}
	}
	return nil
}

func hasVolumeMount(volumeMounts []v1.VolumeMount, name string) bool {
	for _, vm := range volumeMounts {
		if vm.Name == name {
			return true
		}
	}
	return false
}

func applyResource(builder *features.FeatureBuilder, file string) *features.FeatureBuilder {
	return builder.
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := decoder.DecodeEachFile(
				ctx, os.DirFS("test"), file,
				decoder.CreateHandler(cfg.Client().Resources()),
				decoder.MutateNamespace(cfg.Namespace()),
			)
			require.NoError(t, err)

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := decoder.DecodeEachFile(
				ctx, os.DirFS("test"), file,
				decoder.DeleteHandler(cfg.Client().Resources()),
				decoder.MutateNamespace(cfg.Namespace()),
			)
			require.NoError(t, err)

			return ctx
		})
}
