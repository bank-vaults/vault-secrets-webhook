# vault-secrets-webhook

A Kubernetes mutating webhook that makes direct secret injection into Pods possible

**Homepage:** <https://bank-vaults.dev>

This chart will install a mutating admission webhook, that injects an executable to containers in Pods which than can request secrets from Vault through environment variable definitions. Also, it can inject statically into ConfigMaps, Secrets, and CustomResources.

## Before you start

Before you install this chart you must create a namespace for it, this is due to the order in which the resources in the charts are applied (Helm collects all of the resources in a given Chart and it's dependencies, groups them by resource type, and then installs them in a predefined order (see [here](https://github.com/helm/helm/blob/release-2.10/pkg/tiller/kind_sorter.go#L29) - Helm 2.10).

The `MutatingWebhookConfiguration` gets created before the actual backend Pod which serves as the webhook itself, Kubernetes would like to mutate that pod as well, but it is not ready to mutate yet (infinite recursion in logic).

## Using External Vault Instances

You will need to add the following annotations to the resources that you wish to mutate:

```yaml
vault.security.banzaicloud.io/vault-addr: https://[URL FOR VAULT]
vault.security.banzaicloud.io/vault-path: [Auth path]
vault.security.banzaicloud.io/vault-role: [Auth role]
vault.security.banzaicloud.io/vault-skip-verify: "true" # Container is missing Trusted Mozilla roots too.
```

Be mindful how you reference Vault secrets itself. For KV v2 secrets, you will need to add the /data/ to the path of the secret.

```
PS C:\> vault kv get kv/rax/test
====== Metadata ======
Key              Value
---              -----
created_time     2019-09-21T16:55:26.479739656Z
deletion_time    n/a
destroyed        false
version          1

=========== Data ===========
Key                    Value
---                    -----
MYSQL_PASSWORD         3xtr3ms3cr3t
MYSQL_ROOT_PASSWORD    s3cr3t
```

The secret shown above is referenced like this:

```
vault:[ENGINE]/data/[SECRET_NAME]#KEY
vault:kv/rax/data/test#MYSQL_PASSWORD
```

If you want to use a specific key version, you can append it after the key so it becomes like this:

`vault:kv/rax/data/test#MYSQL_PASSWORD#1`

Omitting the version will tell Vault to pull the latest version.

## Installing the Chart

**In case of the K8s version is lower than 1.15 the namespace where you install the webhook must have a label of `name` with the namespace name as the label value, so the `namespaceSelector` in the `MutatingWebhookConfiguration` can skip the namespace of the webhook, so no self-mutation takes place. If the K8s version is 1.15 at least, the default `objectSelector` will prevent the self-mutation (you don't have to configure anything) and you are free to install to any namespace of your choice.**.

```bash
# You have to do this only in case you are not using Helm 3.2 or later and Kubernetes 1.15 or later.
WEBHOOK_NS=${WEBHOOK_NS:-vswh}
kubectl create namespace "${WEBHOOK_NS}"
kubectl label ns "${WEBHOOK_NS}" name="${WEBHOOK_NS}"
```

```bash
$ helm repo add banzaicloud-stable https://kubernetes-charts.banzaicloud.com/
$ helm repo update
```

```bash
$ helm upgrade --namespace vswh --install vswh banzaicloud-stable/vault-secrets-webhook --create-namespace
```

**NOTE**: `--wait` is sometimes necessary because of some Helm timing issues, please see [this issue](https://github.com/banzaicloud/banzai-charts/issues/888).

### Openshift 4.3

For security reasons, the `runAsUser` must be in the range between 1000570000 and 1000579999. By setting the value of `securityContext.runAsUser` to "", OpenShift chooses a valid User.

```bash
$ helm upgrade --namespace vswh --install vswh banzaicloud-stable/vault-secrets-webhook --set-string securityContext.runAsUser="" --create-namespace
```

### About GKE Private Clusters

When Google configures the control plane for private clusters, they automatically configure VPC peering between your Kubernetes cluster’s network in a separate Google managed project.

The auto-generated rules **only** open ports 10250 and 443 between masters and nodes. This means that to use the webhook component with a GKE private cluster, you must configure an additional firewall rule to allow your masters CIDR to access your webhook pod using the port 8443.

You can read more information on how to add firewall rules for the GKE control plane nodes in the [GKE docs](https://cloud.google.com/kubernetes-engine/docs/how-to/private-clusters#add_firewall_rules).

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| replicaCount | int | `2` | Number of replicas |
| debug | bool | `false` | Debug logs for webhook |
| certificate.useCertManager | bool | `false` | Should request cert-manager for getting a new CA and TLS certificate |
| certificate.servingCertificate | string | `nil` | Should use an already externally defined Certificate by cert-manager |
| certificate.generate | bool | `true` | Should a new CA and TLS certificate be generated for the webhook |
| certificate.server.tls.crt | string | `nil` | Base64 encoded TLS certificate signed by the CA |
| certificate.server.tls.key | string | `nil` | Base64 encoded private key of TLS certificate signed by the CA |
| certificate.ca.crt | string | `nil` | Base64 encoded CA certificate |
| certificate.extraAltNames | list | `[]` | Use extra names if you want to use the webhook via an ingress or a loadbalancer |
| certificate.caLifespan | int | `3650` | The number of days from the creation of the CA certificate until it expires |
| certificate.certLifespan | int | `365` | The number of days from the creation of the TLS certificate until it expires |
| image.repository | string | `"ghcr.io/bank-vaults/vault-secrets-webhook"` | Image repo that contains the admission server |
| image.tag | string | `""` | Image tag |
| image.pullPolicy | string | `"IfNotPresent"` | Image pull policy |
| image.imagePullSecrets | list | `[]` | Image pull secrets for private repositories |
| service.name | string | `"vault-secrets-webhook"` | Webhook service name |
| service.type | string | `"ClusterIP"` | Webhook service type |
| service.externalPort | int | `443` | Webhook service external port |
| service.internalPort | int | `8443` | Webhook service internal port |
| service.annotations | object | `{}` | Webhook service annotations, e.g. if type is AWS LoadBalancer and you want to add security groups |
| ingress.enabled | bool | `false` | Webhook ingress enabled |
| ingress.annotations | object | `{}` | Webhook ingress annotations |
| ingress.host | string | `""` | Webhook ingress host |
| webhookClientConfig.useUrl | bool | `false` | Use url if webhook should be contacted over loadbalancer or ingress instead of service object. |
| webhookClientConfig.url | string | `"https://example.com"` | Set the url how the webhook should be contacted (including protocol https://) |
| vaultEnv.repository | string | `"ghcr.io/bank-vaults/vault-env"` | Image repo for the vault-env container |
| vaultEnv.tag | string | `"v1.21.1"` | Image tag for the vault-env container |
| env.VAULT_IMAGE | string | `"hashicorp/vault:1.14.1"` | Vault image |
| env.DEFAULT_IMAGE_PULL_SECRET | string | `""` | Used when the pod that should get secret injected does not specify an imagePullSecret |
| env.DEFAULT_IMAGE_PULL_SECRET_NAMESPACE | string | `""` | Used when the pod that should get secret injected does not specify an imagePullSecret |
| env.DEFAULT_IMAGE_PULL_SECRET_SERVICE_ACCOUNT | string | `""` | Used when the pod that should get secret injected does not specify an imagePullSecret |
| env.VAULT_CLIENT_TIMEOUT | string | `"10s"` | Define the webhook's timeout for Vault communication, if not defined individually in resources by annotations |
| env.VAULT_ROLE | string | `""` | Define the webhook's role in Vault used for authentication, if not defined individually in resources by annotations |
| env.VAULT_ENV_CPU_REQUEST | string | `"50m"` | Cpu requests for init-containers vault-env and copy-vault-env |
| env.VAULT_ENV_CPU_LIMIT | string | `"250m"` | Cpu limits for init-containers vault-env and copy-vault-env |
| env.VAULT_ENV_MEMORY_REQUEST | string | `"64Mi"` | Memory requests for init-containers vault-env and copy-vault-env |
| env.VAULT_ENV_MEMORY_LIMIT | string | `"64Mi"` | Memory limits for init-containers vault-env and copy-vault-env |
| env.VAULT_ENV_LOG_SERVER | string | `""` | Define remote log server for vault-env |
| initContainers | list | `[]` | Containers to run before the app containers are started |
| metrics.enabled | bool | `false` | Enable metrics |
| metrics.port | int | `8443` | Metrics endpoint |
| metrics.serviceMonitor.enabled | bool | `false` | Enable service monitor |
| metrics.serviceMonitor.scheme | string | `"https"` | Service monitor scheme |
| metrics.serviceMonitor.tlsConfig.insecureSkipVerify | bool | `true` | Skip TLS check for service monitor |
| securityContext | object | `{"allowPrivilegeEscalation":false,"runAsUser":65534}` | Container security context for webhook deployment |
| podSecurityContext | object | `{}` | Pod security context for webhook deployment |
| volumes | list | `[]` | Extra volume definitions |
| volumeMounts | list | `[]` | Extra volume mounts |
| podAnnotations | object | `{}` | Extra annotations to add to pod metadata |
| labels | object | `{}` | Extra labels to add to the deployment and pods |
| resources | object | `{}` | Resources to request |
| nodeSelector | object | `{}` | Node selector to use |
| tolerations | list | `[]` | Tolerations to add |
| affinity | object | `{}` | Affinities to use |
| topologySpreadConstraints | object | `{}` | TopologySpreadConstraints to add |
| priorityClassName | string | `""` | Assign a PriorityClassName to pods if set |
| rbac.psp.enabled | bool | `false` | Use pod security policy |
| rbac.authDelegatorRole.enabled | bool | `false` | Bind `system:auth-delegator` to the ServiceAccount |
| serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| serviceAccount.labels | object | `{}` | Labels to add to the service account |
| serviceAccount.annotations | object | `{}` | To enable GKE workload identity, use for example `iam.gke.io/gcp-service-account: gsa@project.iam.gserviceaccount.com`. |
| serviceAccount.name | string | `""` | If not set and create is true, a name is generated using the fullname template |
| deployment.strategy | object | `{}` | Rolling strategy for webhook deployment |
| customResourceMutations | list | `[]` | List of CustomResources to inject values from Vault, for example: ["ingresses", "servicemonitors"] |
| customResourcesFailurePolicy | string | `"Ignore"` |  |
| configMapMutation | bool | `false` | Enable injecting values from Vault to ConfigMaps. This can cause issues when used with Helm, so it is disabled by default. |
| secretsMutation | bool | `true` | Enable injecting values from Vault to Secrets. Set to `false` in order to prevent secret values from being persisted in Kubernetes. |
| configMapFailurePolicy | string | `"Ignore"` |  |
| podsFailurePolicy | string | `"Ignore"` |  |
| secretsFailurePolicy | string | `"Ignore"` |  |
| apiSideEffectValue | string | `"NoneOnDryRun"` | Webhook sideEffect value |
| namespaceSelector | object | `{"matchExpressions":[{"key":"kubernetes.io/metadata.name","operator":"NotIn","values":["kube-system"]}]}` | Namespace selector to use, will limit webhook scope |
| objectSelector | object | `{}` | Object selector to use, will limit webhook scope (K8s version 1.15+) |
| secrets | object | `{"namespaceSelector":{},"objectSelector":{}}` | Object and namespace selector for secrets (overrides `objectSelector`); Requires K8s 1.15+ |
| pods | object | `{"namespaceSelector":{},"objectSelector":{}}` | Object and namespace selector for pods (overrides `objectSelector`); Requires K8s 1.15+ |
| configMaps | object | `{"namespaceSelector":{},"objectSelector":{}}` | Object and namespace selector for configmaps (overrides `objectSelector`); Requires K8s 1.15+ |
| customResources | object | `{"namespaceSelector":{},"objectSelector":{}}` | Object and namespace selector for custom resources (overrides `objectSelector`); Requires K8s 1.15+ |
| podDisruptionBudget.enabled | bool | `true` | Enables PodDisruptionBudget |
| podDisruptionBudget.minAvailable | int | `1` | Represents the number of Pods that must be available (integer or percentage) |
| timeoutSeconds | bool | `false` | Webhook timeoutSeconds value |
| hostNetwork | bool | `false` | Allow pod to use the node network namespace |
| dnsPolicy | string | `""` | then pods with webhooks must set the dnsPolicy to "ClusterFirstWithHostNet" |
| kubeVersion | string | `""` | Override cluster version |

### Certificate options

There are the following options for suppling the webhook with CA and TLS certificates.

#### Generate (default)

The default option is to let helm generate the CA and TLS certificates on deploy time.

This will renew the certificates on each deployment.

```
certificate:
    generate: true
```

#### Manually supplied

Another option is to generate everything manually and specify the TLS `crt` and `key` plus the CA `crt` as values.
These values need to be base64 encoded x509 certificates.

```yaml
certificate:
  generate: false
  server:
    tls:
      crt: LS0tLS1...
      key: LS0tLS1...
  ca:
    crt: LS0tLS1...
```

#### Using cert-manager

If you use cert-manager in your cluster, you can instruct cert-manager to manage everything.
The following options will let cert-manager generate TLS `certificate` and `key` plus the CA `certificate`.

```yaml
certificate:
  generate: false
  useCertManager: true
```
