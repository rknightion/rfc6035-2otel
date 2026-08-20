# syntax=docker/dockerfile:1

FROM golang:1.27.0-bookworm@sha256:484ef6066fa69acb059fdfeda7ba2b8f7391f2ef6abc6f9b8411e669ebd56466 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
      -o /out/rfc6035-2otel ./cmd/rfc6035-2otel

# The static runtime supplies the CA bundle required for OTLP/HTTPS and has no
# shell or package manager. UID/GID 65532 is its nonroot identity.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:1b7b9f0f0e0a1d2155f531db587cc48ec26aaf97ab64364225f5bf18a054e66a
COPY --from=build /out/rfc6035-2otel /usr/local/bin/rfc6035-2otel
USER 65532:65532
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/usr/local/bin/rfc6035-2otel", "-healthcheck", "127.0.0.1:5060"]
ENTRYPOINT ["/usr/local/bin/rfc6035-2otel"]
CMD []
