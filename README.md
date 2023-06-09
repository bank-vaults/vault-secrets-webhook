# Vault Secrets Webhook

[![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/bank-vaults/vault-secrets-webhook/ci.yaml?style=flat-square)](https://github.com/bank-vaults/vault-secrets-webhook/actions/workflows/ci.yaml)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/bank-vaults/vault-secrets-webhook/badge?style=flat-square)](https://api.securityscorecards.dev/projects/github.com/bank-vaults/vault-secrets-webhook)

**A Kubernetes mutating webhook that makes direct secret injection into Pods possible.**

## Documentation

The official documentation for the operator is available at [https://bank-vaults.dev](https://bank-vaults.dev/docs/operator/).

## Development

**For an optimal developer experience, it is recommended to install [Nix](https://nixos.org/download.html) and [direnv](https://direnv.net/docs/installation.html).**

_Alternatively, install [Go](https://go.dev/dl/) on your computer then run `make deps` to install the rest of the dependencies._

Make sure Docker is installed with Compose and Buildx.

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
make test-acceptance
```

Run the linter:

```shell
make lint
```

Some linter violations can automatically be fixed:

```shell
make fmt
```

Build artifacts locally:

```shell
make artifacts
```

Once you are done either stop or tear down dependencies:

```shell
make stop

# OR

make down
```

## License

The project is licensed under the [Apache 2.0 License](LICENSE).
