# MinIO 不再发布公开的镜像和二进制：从固定版本的源码构建（版本和 deploy/testenv-ci.sh 一致）。
FROM golang:1.27.3-alpine AS build
RUN apk add --no-cache git
ENV CGO_ENABLED=0
RUN git clone -q --depth 1 --branch RELEASE.2025-04-22T22-12-26Z https://github.com/minio/minio /src/minio \
 && cd /src/minio && go build -o /minio .
RUN git clone -q --depth 1 --branch RELEASE.2025-04-16T18-13-26Z https://github.com/minio/mc /src/mc \
 && cd /src/mc && go build -o /mc .

FROM alpine:3.22
COPY --from=build /minio /mc /usr/local/bin/
