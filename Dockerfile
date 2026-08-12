# syntax=docker/dockerfile:1

FROM golang:1.26.5-bookworm@sha256:53eeac89074db483fdf0ab3be1df32bf6e47562263d2d0d6baa7f26acb4957dd AS build
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
FROM gcr.io/distroless/static-debian12:nonroot@sha256:f5b485ea962d9bd1186b2f6b3a061191539b905b82ec395de78cbfae51f20e35
COPY --from=build /out/rfc6035-2otel /usr/local/bin/rfc6035-2otel
USER 65532:65532
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/usr/local/bin/rfc6035-2otel", "-healthcheck", "127.0.0.1:5060"]
ENTRYPOINT ["/usr/local/bin/rfc6035-2otel"]
CMD []
