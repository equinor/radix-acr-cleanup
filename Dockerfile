FROM docker.io/golang:1.22-alpine3.20 AS builder

ENV CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /src

# Install project dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy project code
COPY . .

RUN go build -ldflags="-s -w" -o /build/radix-acr-cleanup ./cmd/acr-cleanup/.

FROM mcr.microsoft.com/azure-cli:2.62.0

# upgrade packages with vulnerabilities in mcr.microsoft.com/azure-cli:<tag>
# check if upgrades are necessary (snyk container test mcr.microsoft.com/azure-cli:<tag>)
# when updating to a new tag of mcr.microsoft.com/azure-cli
RUN apk update && \
    apk upgrade

ARG UID=1000
ARG GID=1000

RUN addgroup -S -g $GID acr-cleanup
RUN adduser -S -u $UID -s /bin/sh -G acr-cleanup acr-cleanup

WORKDIR /app/
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/radix-acr-cleanup /app/radix-acr-cleanup
COPY --from=builder /src/run_acr_cleanup.sh /app/run_acr_cleanup.sh

ENV TENANT=3aa4a235-b6e2-48d5-9195-7fcf05b459b0 \
    AZURE_CREDENTIALS=/app/.azure/sp_credentials.json

EXPOSE 8080
RUN chmod +x /app/run_acr_cleanup.sh
USER 1000
CMD ["./run_acr_cleanup.sh"]
