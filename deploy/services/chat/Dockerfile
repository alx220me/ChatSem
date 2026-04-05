FROM golang:1.24-alpine AS builder
WORKDIR /app

# Copy workspace descriptor and shared module
COPY go.work go.work.sum ./
COPY shared/ shared/
# Copy all service go.mod files so go.work can resolve the workspace
COPY services/chat/go.mod services/chat/go.sum services/chat/
COPY services/auth/go.mod services/auth/go.sum services/auth/
COPY services/admin/go.mod services/admin/go.sum services/admin/
# Copy full source of the service being built
COPY services/chat/ services/chat/

# Build with CGO disabled for static binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /service \
    ./services/chat/cmd/main.go

# Minimal distroless runtime image
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /service /service
EXPOSE 8080
ENTRYPOINT ["/service"]
