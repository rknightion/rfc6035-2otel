# syntax=docker/dockerfile:1

FROM golang:1.27.3-bookworm@sha256:05237276236793ebedaf0b87dab86add9ec9adc66ebef2dbc3107c18254cde1c AS build
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
FROM gcr.io/distroless/static-debian13:nonroot@sha256:18f4cc26c9ffa7f20a359298be7deadac5c607c8e03a02b8119f1a5fc2f82c99
COPY --from=build /out/rfc6035-2otel /usr/local/bin/rfc6035-2otel
USER 65532:65532
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/usr/local/bin/rfc6035-2otel", "-healthcheck", "127.0.0.1:5060"]
ENTRYPOINT ["/usr/local/bin/rfc6035-2otel"]
CMD []
