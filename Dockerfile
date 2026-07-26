# 第一阶段：编译
FROM golang:1.24-alpine AS builder

# 设置 Go 环境
ENV GOPROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

WORKDIR /app

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 编译
RUN go build -ldflags="-s -w" -o qatest-server .

# 第二阶段：运行
FROM alpine:3.20

# 安装 ca-certificates（HTTPS 请求需要）
RUN apk add --no-cache ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

WORKDIR /app

# 从编译阶段复制二进制文件
COPY --from=builder /app/qatest-server .
COPY --from=builder /app/.env.example .env

# 创建数据目录，并改为非 root 用户运行（P1：最小权限）
RUN addgroup -S app && adduser -S -u 1000 -G app app \
    && mkdir -p /app/data /app/logs \
    && chown -R app:app /app
USER 1000

# 暴露端口
EXPOSE 3000

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3000/api/auth/login || exit 1

# 启动
CMD ["./qatest-server"]
