FROM golang:1.23-alpine

RUN addgroup -S sandbox && adduser -S sandbox -G sandbox

ENV GOFLAGS=""
ENV GOPROXY=off

WORKDIR /workspace

USER sandbox
