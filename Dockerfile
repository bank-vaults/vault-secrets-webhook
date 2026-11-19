FROM --platform=$BUILDPLATFORM tonistiigi/xx:1.10.0@sha256:40fe21f44bb8cddf1eb1226db190d0a984118f62ee02a91b77d99448d28264a6 AS xx

FROM --platform=$BUILDPLATFORM golang:1.26-alpine3.22@sha256:727cfc3c40be55cd1bc9a4a059406b28a059857e3be752aa9d09531e12c20c56 AS builder

# xx ships its cross-compilation helpers across the whole filesystem
# hadolint ignore=DL3067
COPY --from=xx / /

RUN apk add --update --no-cache ca-certificates make git curl clang lld

ARG TARGETPLATFORM

RUN xx-apk --update --no-cache add musl-dev gcc

RUN xx-go --wrap

WORKDIR /usr/local/src/vault-secrets-webhook

ARG GOPROXY

ENV CGO_ENABLED=0

COPY go.* ./
RUN go mod download

COPY . .

RUN go build -o /usr/local/bin/vault-secrets-webhook .
RUN xx-verify /usr/local/bin/vault-secrets-webhook


FROM alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

RUN apk add --update --no-cache ca-certificates tzdata libcap

COPY --from=builder /usr/local/bin/vault-secrets-webhook /usr/local/bin/vault-secrets-webhook

USER 65534

ENTRYPOINT ["vault-secrets-webhook"]
