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

FROM spritsail/alpine:3.14

ARG VERSION

LABEL maintainer="frebib <cmd-exporter@frebib.net>" \
      org.label-schema.vendor="frebib" \
      org.label-schema.name="Prometheus command exporter" \
      org.label-schema.url="https://github.com/frebib/cmd-exporter" \
      org.label-schema.description="Prometheus exporter for arbitrary commands and scripts" \
      org.label-schema.version=${VERSION}

COPY --from=build /tmp/build/cmd-exporter /usr/bin/

EXPOSE 9654
CMD ["/usr/bin/cmd-exporter"]
