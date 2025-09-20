FROM golang:1.20-alpine AS builder

WORKDIR /app

# 复制源代码
COPY . .

# 构建应用
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o proxy-app .

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/proxy-app .
COPY --from=builder /app/static ./static
COPY --from=builder /app/templates ./templates

# 暴露端口
EXPOSE 8080

# 设置环境变量
ENV PORT=8080
ENV PROXY_CONFIGS="Google:https://www.google.com,GitHub:https://github.com,Baidu:https://www.baidu.com"

CMD ["./proxy-app"]