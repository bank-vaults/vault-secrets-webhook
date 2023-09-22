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
// +build e2e

package e2e

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/e2e-framework/klient/conf"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support/kind"
	"sigs.k8s.io/e2e-framework/third_party/helm"
)

// Upgrade this when a new version is released
const vaultOperatorVersion = "1.20.0"

var testenv env.Environment

func TestMain(m *testing.M) {
	// See https://github.com/kubernetes-sigs/e2e-framework/issues/269
	// testenv = env.New()
	testenv = &reverseFinishEnvironment{Environment: env.New()}

	if os.Getenv("LOG_VERBOSE") == "true" {
		flags := flag.NewFlagSet("", flag.ContinueOnError)
		klog.InitFlags(flags)
		flags.Parse([]string{"-v", "4"})
	}
	log.SetLogger(klog.NewKlogr())

	bootstrap := strings.ToLower(os.Getenv("BOOTSTRAP")) != "false"
	useRealCluster := !bootstrap || strings.ToLower(os.Getenv("USE_REAL_CLUSTER")) == "true"

	// Set up cluster
	if useRealCluster {
		path := conf.ResolveKubeConfigFile()
		cfg := envconf.NewWithKubeConfig(path)

		if context := os.Getenv("USE_CONTEXT"); context != "" {
			cfg.WithKubeContext(context)
		}

		// See https://github.com/kubernetes-sigs/e2e-framework/issues/269
		// testenv = env.NewWithConfig(cfg)
		testenv = &reverseFinishEnvironment{Environment: env.NewWithConfig(cfg)}
	} else {
		clusterName := envconf.RandomName("vault-secrets-webhook-test", 32)

		kindCluster := kind.NewProvider()
		if v := os.Getenv("KIND_K8S_VERSION"); v != "" {
			kindCluster.WithOpts(kind.WithImage("kindest/node:" + v))
			testenv.Setup(envfuncs.CreateClusterWithConfig(kindCluster, clusterName, "kind.yaml"))
		} else {
			testenv.Setup(envfuncs.CreateCluster(kindCluster, clusterName))
		}

		testenv.Finish(envfuncs.DestroyCluster(clusterName))

		if image := os.Getenv("LOAD_IMAGE"); image != "" {
			testenv.Setup(envfuncs.LoadDockerImageToCluster(clusterName, image))
		}

		if imageArchive := os.Getenv("LOAD_IMAGE_ARCHIVE"); imageArchive != "" {
			testenv.Setup(envfuncs.LoadImageArchiveToCluster(clusterName, imageArchive))
		}
	}

	if bootstrap {
		// Install vault-operator
		testenv.Setup(installVaultOperator)
		testenv.Finish(uninstallVaultOperator, envfuncs.DeleteNamespace("vault-operator"))

		testenv.Setup(envfuncs.CreateNamespace("vault-secrets-webhook"), installVaultSecretsWebhook)
		testenv.Finish(uninstallVaultSecretsWebhook, envfuncs.DeleteNamespace("vault-secrets-webhook"))

		// Set up test namespace
		// ns := envconf.RandomName("webhook-test", 16)
		// testenv.Setup(envfuncs.CreateNamespace(ns))
		// testenv.Finish(envfuncs.DeleteNamespace(ns))

		// Unsealing and Vault access only works in the default namespace at the moment
		testenv.Setup(useNamespace("default"))

		testenv.Setup(installVault, waitForVaultTLS)
		testenv.Finish(uninstallVault)
	} else {
		// Unsealing and Vault access only works in the default namespace at the moment
		testenv.Setup(useNamespace("default"))
	}

	os.Exit(testenv.Run(m))
}

func installVaultOperator(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
	manager := helm.New(cfg.KubeconfigFile())

	err := manager.RunInstall(
		helm.WithName("vault-operator"), // This is weird that ReleaseName works differently, but it is what it is
		helm.WithChart("oci://ghcr.io/bank-vaults/helm-charts/vault-operator"),
		helm.WithNamespace("vault-operator"),
		helm.WithArgs("--create-namespace"),
		helm.WithVersion(vaultOperatorVersion),
		helm.WithWait(),
		helm.WithTimeout("2m"),
	)
	if err != nil {
		return ctx, fmt.Errorf("installing vault-operator: %w", err)
	}

	return ctx, nil
}

func uninstallVaultOperator(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
	manager := helm.New(cfg.KubeconfigFile())

	err := manager.RunUninstall(
		helm.WithName("vault-operator"),
		helm.WithNamespace("vault-operator"),
		helm.WithWait(),
		helm.WithTimeout("2m"),
	)
	if err != nil {
		return ctx, fmt.Errorf("uninstalling vault-operator: %w", err)
	}

	return ctx, nil
}

func installVaultSecretsWebhook(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
	manager := helm.New(cfg.KubeconfigFile())

	version := "latest"
	if v := os.Getenv("WEBHOOK_VERSION"); v != "" {
		version = v
	}

	chart := "../deploy/charts/vault-secrets-webhook/"
	if v := os.Getenv("HELM_CHART"); v != "" {
		chart = v
	}

	err := manager.RunInstall(
		helm.WithName("vault-secrets-webhook"), // This is weird that ReleaseName works differently, but it is what it is
		helm.WithChart(chart),
		helm.WithNamespace("vault-secrets-webhook"),
		helm.WithArgs("-f", "deploy/vault-secrets-webhook/values.yaml", "--set", "image.tag="+version),
		helm.WithWait(),
		helm.WithTimeout("2m"),
	)
	if err != nil {
		return ctx, fmt.Errorf("installing vault-secrets-webhook: %w", err)
	}

	return ctx, nil
}

func uninstallVaultSecretsWebhook(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
	manager := helm.New(cfg.KubeconfigFile())

	err := manager.RunUninstall(
		helm.WithName("vault-secrets-webhook"),
		helm.WithNamespace("vault-secrets-webhook"),
		helm.WithWait(),
		helm.WithTimeout("2m"),
	)
	if err != nil {
		return ctx, fmt.Errorf("uninstalling vault-secrets-webhook: %w", err)
	}

	return ctx, nil
}

func useNamespace(ns string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		cfg.WithNamespace(ns)

		return ctx, nil
	}
}

func installVault(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
	r, err := resources.New(cfg.Client().RESTConfig())
	if err != nil {
		return ctx, err
	}

	err = decoder.DecodeEachFile(
		ctx, os.DirFS("deploy/vault"), "*",
		decoder.CreateHandler(r),
		decoder.MutateNamespace(cfg.Namespace()),
	)
	if err != nil {
		return ctx, err
	}

	statefulSets := &appsv1.StatefulSetList{
		Items: []appsv1.StatefulSet{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "vault", Namespace: cfg.Namespace()},
			},
		},
	}

	// wait for the statefulSet to become available
	err = wait.For(conditions.New(r).ResourcesFound(statefulSets), wait.WithTimeout(1*time.Minute))
	if err != nil {
		return ctx, err
	}

	time.Sleep(5 * time.Second)

	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "vault-0", Namespace: cfg.Namespace()},
	}

	// wait for the pod to become available
	err = wait.For(conditions.New(r).PodReady(&pod), wait.WithTimeout(1*time.Minute))
	if err != nil {
		return ctx, err
	}

	return ctx, nil
}

func waitForVaultTLS(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
	vaultTLSSecrets := &v1.SecretList{
		Items: []v1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "vault-tls", Namespace: cfg.Namespace()},
			},
		},
	}

	// wait for the vault-tls secret to become available
	err := wait.For(conditions.New(cfg.Client().Resources()).ResourcesFound(vaultTLSSecrets), wait.WithTimeout(1*time.Minute))
	if err != nil {
		return ctx, err
	}

	return ctx, nil
}

func uninstallVault(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
	r, err := resources.New(cfg.Client().RESTConfig())
	if err != nil {
		return ctx, err
	}

	err = decoder.DecodeEachFile(
		ctx, os.DirFS("deploy/vault"), "*",
		decoder.DeleteHandler(r),
		decoder.MutateNamespace(cfg.Namespace()),
	)

	if err != nil {
		return ctx, err
	}

	return ctx, nil
}

type reverseFinishEnvironment struct {
	env.Environment

	finishFuncs []env.Func
}

// Finish registers funcs that are executed at the end of the test suite in a reverse order.
func (e *reverseFinishEnvironment) Finish(f ...env.Func) env.Environment {
	e.finishFuncs = append(f[:], e.finishFuncs...)

	return e
}

// Run launches the test suite from within a TestMain.
func (e *reverseFinishEnvironment) Run(m *testing.M) int {
	e.Environment.Finish(e.finishFuncs...)

	return e.Environment.Run(m)
}
