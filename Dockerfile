FROM golang:1.19-alpine AS builder

WORKDIR /app

# 复制go.mod和go.sum文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -o proxy-app .

# 使用轻量级的alpine镜像
FROM alpine:latest

WORKDIR /app

# 从builder阶段复制编译好的应用
COPY --from=builder /app/proxy-app .

# 复制静态文件和模板
COPY --from=builder /app/static ./static
COPY --from=builder /app/templates ./templates

# 暴露端口
EXPOSE 8080

# 设置环境变量
ENV PORT=8080
ENV PROXY_CONFIGS="Google:https://www.google.com,GitHub:https://github.com,Baidu:https://www.baidu.com"

# 运行应用
CMD ["./proxy-app"]