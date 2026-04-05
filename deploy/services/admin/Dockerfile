FROM golang:1.24-alpine AS builder
WORKDIR /app

COPY go.work go.work.sum ./
COPY shared/ shared/
COPY services/chat/go.mod services/chat/go.sum services/chat/
COPY services/auth/go.mod services/auth/go.sum services/auth/
COPY services/admin/go.mod services/admin/go.sum services/admin/
COPY services/admin/ services/admin/

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /service \
    ./services/admin/cmd/main.go

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /service /service
EXPOSE 8082
ENTRYPOINT ["/service"]
