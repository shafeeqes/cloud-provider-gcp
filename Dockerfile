############# builder            #############
FROM golang:1.20.8 AS builder

WORKDIR /go/src/github.com/gardener/cloud-provider-gcp
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install \
  -mod=vendor \
  ./...

############# cloud-provider-gcp #############
FROM gcr.io/distroless/static-debian11:nonroot AS cloud-provider-gcp

COPY --from=builder /go/bin/gcp-cloud-controller-manager /gcp-cloud-controller-manager

WORKDIR /

ENTRYPOINT ["/gcp-cloud-controller-manager"]
