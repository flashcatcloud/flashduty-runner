FROM golang:1.24-alpine AS build
ARG VERSION="dev"
ARG TARGETARCH

# Set the working directory
WORKDIR /build

# Install git for version info
RUN --mount=type=cache,target=/var/cache/apk \
    apk add git

# Build the runner
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 GOARCH=${TARGETARCH} go build \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.GitCommit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown) -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /bin/flashduty-runner ./cmd

# Make a stage to run the app
FROM gcr.io/distroless/base-debian12

# Set the working directory
WORKDIR /app

# Copy the binary from the build stage
COPY --from=build /bin/flashduty-runner .

# Set environment variables
ENV FLASHDUTY_API_KEY=""
ENV FLASHDUTY_API_URL="wss://api.flashcat.cloud/runner/ws"
ENV FLASHDUTY_RUNNER_NAME=""
ENV FLASHDUTY_WORKSPACE_ROOT="/workspace"
ENV FLASHDUTY_AUTO_UPDATE="false"

# Create workspace directory
VOLUME ["/workspace"]

# Set the entrypoint
ENTRYPOINT ["/app/flashduty-runner"]

# Default command
CMD ["run"]
