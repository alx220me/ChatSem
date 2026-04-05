FROM golang:1.24-alpine AS builder
WORKDIR /app

# Copy workspace descriptor and shared module
COPY go.work go.work.sum ./
COPY shared/ shared/
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
