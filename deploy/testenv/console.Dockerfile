# 测试环境的 console-server。构建上下文是 tianting-console-server 仓库。
FROM golang:1.27.3-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /console-server ./cmd/console-server

FROM alpine:3.22
COPY --from=build /console-server /usr/local/bin/console-server
ENTRYPOINT ["console-server"]
