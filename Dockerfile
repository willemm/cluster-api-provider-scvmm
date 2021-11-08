# Build the manager binary
FROM golang:1.17 as builder

RUN curl http://pr-art.europe.stater.corp/artifactory/auto-local/certs/pr-root.cer | sed -e "s/\r//g" > /usr/local/share/ca-certificates/pr-root.crt \
 && update-ca-certificates

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN GOPROXY=https://proxy.golang.org go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/

# Build
RUN GOPROXY=https://proxy.golang.org CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM distroless/static:nonroot
ENV SCRIPT_DIR=/scripts
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532
COPY scripts/ scripts/

ENTRYPOINT ["/manager"]
