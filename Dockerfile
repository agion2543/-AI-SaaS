FROM golang:1.22-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server \
    && CGO_ENABLED=0 GOOS=linux go build -o schema-migrate ./cmd/schema-migrate

FROM alpine:3.20
# tzdata lets the Go runtime honour TZ=Asia/Shanghai (time.Local); without
# it the container falls back to UTC even when TZ is set.
RUN apk add --no-cache tzdata
WORKDIR /app
COPY --from=builder /app/server /app/server
COPY --from=builder /app/schema-migrate /app/schema-migrate
COPY --from=builder /app/.env.example /app/.env.example
EXPOSE 8080
CMD ["/app/server"]
