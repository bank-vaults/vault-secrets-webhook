# Vault Secrets Webhook

[![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/bank-vaults/vault-secrets-webhook/ci.yaml?style=flat-square)](https://github.com/bank-vaults/vault-secrets-webhook/actions/workflows/ci.yaml)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/bank-vaults/vault-secrets-webhook/badge?style=flat-square)](https://api.securityscorecards.dev/projects/github.com/bank-vaults/vault-secrets-webhook)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/7961/badge)](https://www.bestpractices.dev/projects/7961)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/vault-secrets-webhook)](https://artifacthub.io/packages/search?repo=vault-secrets-webhook)

**A Kubernetes mutating webhook that makes direct secret injection into Pods possible.**

## Documentation

The official documentation for the webhook is available at [https://bank-vaults.dev](https://bank-vaults.dev/docs/mutating-webhook/).

## Development

**For an optimal developer experience, it is recommended to install [Nix](https://nixos.org/download.html) and [direnv](https://direnv.net/docs/installation.html).**

_Alternatively, install [Go](https://go.dev/dl/) on your computer then run `make deps` to install the rest of the dependencies._

Make sure Docker is installed with Compose and Buildx.

Fetch required tools:

```shell
make deps
```

Run project dependencies:

```shell
make up
```

Run the webhook:

```shell
make -j run forward
```

Run the test suite:

```shell
make test
make test-e2e-local
```

Run linters:

```shell
make lint # pass -j option to run them in parallel
```

Some linter violations can automatically be fixed:

```shell
make fmt
```

Build artifacts locally:

```shell
make artifacts
```

Once you are done, you can tear down project dependencies:

```shell
make down
```

### Running e2e tests

The project comes with an e2e test suite that is mostly self-contained,
but at the very least, you need Docker installed.

By default, the suite launches a [KinD](https://kind.sigs.k8s.io/) cluster, deploys all necessary components and runs the test suite.
This is a good option if you want to run the test suite to make sure everything works. This is also how the CI runs the test suite
(with a few minor differences).

You can run the test suite by running the following commands:

```shell
make test-e2e-local
```

Another way to run the test suite is using an existing cluster.
This may be a better option if you want to debug tests or figure out why something isn't working.

Set up a Kubernetes cluster of your liking. For example, launch a KinD cluster:

```shell
kind create cluster
```

Deploy the necessary components (including the webhook itself):

```shell
garden deploy
```

Run the test suite:

```shell
make BOOTSTRAP=false test-e2e
```

## License

The project is licensed under the [Apache 2.0 License](LICENSE).
