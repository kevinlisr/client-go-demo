# Build the manager binary
FROM golang:1.17 as builder

WORKDIR /workspace

ENV GOPROXY=https://goproxy.io
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY controllers/template/deployment.yml deployment.yml



# Build
RUN GOPROXY=https://goproxy.io CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
#FROM gcr.io/distroless/static:nonroot
FROM ubuntu
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/controllers controllers
USER 65532:65532

ENTRYPOINT ["/manager"]
