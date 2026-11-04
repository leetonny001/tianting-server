# 测试环境的 im-server。版本号构建时注入（AC-OPS-04-6：n1 = 0.3.0，n2 = 0.3.1）。
# 构建上下文是 server/。
FROM golang:1.27.3-alpine AS build
ARG VERSION=dev
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=${VERSION}" -o /im-server ./cmd/im-server

FROM alpine:3.22
RUN adduser -D -u 10001 im && mkdir /data && chown im /data
COPY --from=build /im-server /usr/local/bin/im-server
USER im
# exec：PID 1 必须是 im-server 本身，kill -STOP、kill -9 才打到它。
ENTRYPOINT ["sh", "-c", "im-server migrate up && exec im-server"]
