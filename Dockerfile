FROM --platform=$BUILDPLATFORM tonistiigi/xx:1.4.0@sha256:0cd3f05c72d6c9b038eb135f91376ee1169ef3a330d34e418e65e2a5c2e9c0d4 AS xx

FROM --platform=$BUILDPLATFORM golang:1.22.0-alpine3.18@sha256:2745a45f77ae2e7be569934fa9a111f067d04c767f54577e251d9b101250e46b AS builder

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


FROM alpine:3.19.1@sha256:c5b1261d6d3e43071626931fc004f70149baeba2c8ec672bd4f27761f8e1ad6b

RUN apk add --update --no-cache ca-certificates tzdata libcap

COPY --from=builder /usr/local/bin/vault-secrets-webhook /usr/local/bin/vault-secrets-webhook

USER 65534

ENTRYPOINT ["vault-secrets-webhook"]
