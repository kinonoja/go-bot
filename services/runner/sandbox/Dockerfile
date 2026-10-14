FROM golang:1.23-alpine

RUN addgroup -S sandbox && adduser -S sandbox -G sandbox

ENV CGO_ENABLED=0
ENV GOFLAGS=""
ENV GOPROXY=off

USER root

COPY warm/ /warm/

RUN mkdir -p /opt/gocache && cd /warm && CGO_ENABLED=0 GOFLAGS="" GOPROXY=off GOCACHE=/opt/gocache go build -o /dev/null . && CGO_ENABLED=0 GOFLAGS="" GOPROXY=off GOCACHE=/opt/gocache go test -count=1 ./... && chmod -R a+rX /opt/gocache

WORKDIR /workspace

USER sandbox
