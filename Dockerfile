ARG VERSION=0.1.0

FROM golang:alpine AS build

ARG VERSION

WORKDIR /tmp/build
COPY go.mod go.sum ./
RUN go mod download -x

ARG CGO_ENABLED=0
COPY *.go ./
RUN go build -v -ldflags "-w -s -X main.Version=$VERSION"

# ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

FROM spritsail/alpine:3.20

ARG VERSION

LABEL org.opencontainers.image.authors="frebib <cmd-exporter@frebib.net>" \
      org.opencontainers.image.title="Prometheus command exporter" \
      org.opencontainers.image.url="https://github.com/frebib/cmd-exporter" \
      org.opencontainers.image.description="Prometheus exporter for arbitrary commands and scripts" \
      org.opencontainers.image.version=${VERSION}

COPY --from=build /tmp/build/cmd-exporter /usr/bin/

EXPOSE 9654
CMD ["/usr/bin/cmd-exporter"]
