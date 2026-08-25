# syntax=docker/dockerfile:1.7
FROM golang:1.26.6-alpine3.23@sha256:e57c41c1d5864341031181b0db34b9a537bb5773eb6428e4e5bdaea0f9135406 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal

ARG VERSION=dev
ARG REVISION=unknown
ARG BUILD_DATE=1970-01-01T00:00:00Z
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-s -w -buildid= -X main.version=${VERSION} -X main.revision=${REVISION} -X main.buildDate=${BUILD_DATE}" \
    -o /out/ddae-exporter ./cmd/ddae-exporter

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build --chown=65532:65532 /out/ddae-exporter /ddae-exporter
USER 65532:65532
EXPOSE 9469
ENTRYPOINT ["/ddae-exporter"]
