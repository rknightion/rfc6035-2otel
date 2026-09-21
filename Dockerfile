# syntax=docker/dockerfile:1

FROM golang:1.27.1-bookworm@sha256:648f440f42a0958804efb24df176f806f9d353b41f1c0627f666428e40310f6b AS build
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
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /out/rfc6035-2otel /usr/local/bin/rfc6035-2otel
USER 65532:65532
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/usr/local/bin/rfc6035-2otel", "-healthcheck", "127.0.0.1:5060"]
ENTRYPOINT ["/usr/local/bin/rfc6035-2otel"]
CMD []
