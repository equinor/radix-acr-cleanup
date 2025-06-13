FROM --platform=$BUILDPLATFORM docker.io/golang:1.24-alpine3.22 AS builder

ARG TARGETARCH

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${TARGETARCH}

WORKDIR /src

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -ldflags="-s -w" -o /build/radix-acr-cleanup ./cmd/acr-cleanup/.

# Final stage
FROM mcr.microsoft.com/azure-cli:2.74.0

WORKDIR /app/

COPY --from=builder /build/radix-acr-cleanup /app/radix-acr-cleanup

COPY --from=builder /src/run_acr_cleanup.sh /app/run_acr_cleanup.sh

RUN chmod +x /app/run_acr_cleanup.sh

# built-in nonroot user
USER 65532

CMD ["./run_acr_cleanup.sh"]
