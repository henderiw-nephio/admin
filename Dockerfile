# Build the manager binary
FROM golang:1.18 as builder
WORKDIR /workspace
# Workaround for Private Github REPOs
ARG GITHUB_TOKEN
RUN go env -w GOPRIVATE=github.com/*
RUN git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download
# Copy the go source
COPY cmd/ cmd/
COPY api/ api/
COPY internal/ internal/
COPY pkg/ pkg/
# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager cmd/main.go
# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
#FROM gcr.io/distroless/static:nonroot
FROM alpine:latest
RUN apk add --update && \
    apk add --no-cache openssh && \
    apk add curl && \
    apk add tcpdump && \
    apk add iperf3 &&\
    apk add netcat-openbsd && \
    apk add ethtool && \
    apk add bonding && \
    rm -rf /tmp/*/var/cache/apk/*

RUN curl -sL https://get-gnmic.kmrd.dev | sh
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532
ENTRYPOINT ["/manager"]
