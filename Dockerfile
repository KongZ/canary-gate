FROM --platform=$BUILDPLATFORM golang:1.24.0-bookworm AS builder
ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG VERSION
ARG COMMIT_HASH
ARG BUILD_DATE
ARG LDFLAGS
ENV LDFLAGS="${LDFLAGS} -w -X main.version=${VERSION} -X main.commitHash=${COMMIT_HASH} -X main.buildDate=${BUILD_DATE}"

# Install tools
RUN apt-get update && apt-get -y --no-install-recommends install \
    ca-certificates \
    git \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

# Download modules
WORKDIR /canary-gate
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy files
COPY . .

RUN go install github.com/securego/gosec/v2/cmd/gosec@latest
RUN go install honnef.co/go/tools/cmd/staticcheck@latest
RUN go mod vendor
RUN CGO_ENABLED=0 go vet ./...
RUN CGO_ENABLED=0 staticcheck -f "stylish" ./...
RUN gosec -fmt=text ./...

# Build executable binary
RUN CGO_ENABLED=0 GOOS="$TARGETOS" GOARCH="$TARGETARCH" go build -v -o canary-gate -ldflags="$LDFLAGS" .

################################
# Main image
################################
FROM scratch 
ARG VERSION
ARG COMMIT_HASH
ARG BUILD_DATE
LABEL VERSION=${VERSION}
LABEL COMMIT_HASH=${COMMIT_HASH}
LABEL BUILD_DATE=${BUILD_DATE}
LABEL org.opencontainers.image.source=https://github.com/KongZ/canary-gate
LABEL org.opencontainers.image.description="Canary Gate server for integration with Flagger that offers detailed management of Canary deployments"

ENV VERSION=${VERSION}
ENV LISTEN_ADDRESS=":8080"

# Use an unprivileged user. Don't use named user to avoid PSP error
USER 10001

# Copy files from builder image
COPY --from=builder /canary-gate/canary-gate /canary-gate
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/canary-gate"]
EXPOSE 8080