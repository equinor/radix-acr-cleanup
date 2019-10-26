FROM golang:alpine3.10 as builder

ENV GO111MODULE=on

RUN apk update && \
    apk add ca-certificates  && \
    apk add --no-cache gcc musl-dev && \
    go get -u golang.org/x/lint/golint

WORKDIR /go/src/github.com/equinor/radix-acr-cleanup/

# Install project dependencies
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# run tests and linting
RUN golint `go list ./...` && \
    go vet `go list ./...` && \
    CGO_ENABLED=0 GOOS=linux go test `go list ./...`

# build
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -a -installsuffix cgo -o /usr/local/bin/radix-acr-cleanup ./cmd/acr-cleanup/.

FROM microsoft/azure-cli:2.0.54
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/local/bin/radix-acr-cleanup /usr/local/bin/radix-acr-cleanup
COPY --from=builder /go/src/github.com/equinor/radix-acr-cleanup/run_acr_cleanup.sh /usr/local/bin/run_acr_cleanup.sh

ENV TENANT=3aa4a235-b6e2-48d5-9195-7fcf05b459b0 \
    AZURE_CREDENTIALS=/usr/local/bin/radix-acr-cleanup/.azure/sp_credentials.json

EXPOSE 8080
RUN chmod +x /usr/local/bin/run_acr_cleanup.sh
ENTRYPOINT [ "/usr/local/bin/run_acr_cleanup.sh"]
CMD ["-c"]
