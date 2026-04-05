FROM golang:1.24-alpine AS builder
WORKDIR /app

COPY go.work go.work.sum ./
COPY shared/ shared/
COPY services/auth/ services/auth/

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /service \
    ./services/auth/cmd/main.go

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /service /service
EXPOSE 8081
ENTRYPOINT ["/service"]
