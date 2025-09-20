FROM golang:1.20-alpine AS builder

WORKDIR /app

# 复制源代码
COPY main.go .
COPY go.mod .

# 构建应用
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o proxy-app .

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/proxy-app .

EXPOSE 8080

CMD ["./proxy-app"]