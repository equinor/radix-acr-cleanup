FROM --platform=$BUILDPLATFORM docker.io/golang:1.22-alpine3.20 AS builder
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
FROM mcr.microsoft.com/azure-cli:2.68.0

ARG UID=1000
ARG GID=1000

RUN addgroup -S -g $GID acr-cleanup
RUN adduser -S -u $UID -s /bin/sh -G acr-cleanup acr-cleanup

WORKDIR /app/
COPY --from=builder /build/radix-acr-cleanup /app/radix-acr-cleanup
COPY --from=builder /src/run_acr_cleanup.sh /app/run_acr_cleanup.sh

ENV TENANT=3aa4a235-b6e2-48d5-9195-7fcf05b459b0 \
    AZURE_CREDENTIALS=/app/.azure/sp_credentials.json

EXPOSE 8080
RUN chmod +x /app/run_acr_cleanup.sh
USER 1000
CMD ["./run_acr_cleanup.sh"]
