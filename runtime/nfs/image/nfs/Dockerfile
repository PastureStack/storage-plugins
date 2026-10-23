ARG GO_IMAGE=golang:1.27.3-bookworm@sha256:972b4110ba0f47539a975c31e52c64765e1ff19f2d74469ae4a595c6676e80a5
ARG RUNTIME_IMAGE=alpine:3.23@sha256:1beb0dc0a51de7ff38e3b5274078a2e0b81113ba5c7535e1a03d5913a5edbda3
ARG IMAGE_VERSION=v0.11.0

FROM ${GO_IMAGE} AS runtime-build
ARG IMAGE_VERSION
WORKDIR /src/runtime/nfs
COPY runtime/nfs/ ./
ENV CGO_ENABLED=0
RUN go build \
      -mod=vendor \
      -trimpath \
      -ldflags="-s -w -X main.VERSION=${IMAGE_VERSION}" \
      -o /out/pasturestack-storage-runtime \
      .

FROM ${RUNTIME_IMAGE}
ARG IMAGE_VERSION
LABEL org.opencontainers.image.source="https://github.com/PastureStack/storage-plugins" \
      org.opencontainers.image.description="PastureStack NFS storage driver for the compatible control plane." \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="${IMAGE_VERSION}"
RUN apk add --no-cache \
        bash=5.3.3-r1 \
        ca-certificates=20261020-r0 \
        curl=8.22.0-r0 \
        jq=1.8.2-r0 \
        nfs-utils=2.6.4-r6 \
        rpcbind=1.2.8-r0 \
        util-linux=2.41.6-r1
COPY --from=runtime-build /out/pasturestack-storage-runtime /usr/bin/
COPY runtime/nfs/image/nfs/pasturestack-nfs runtime/nfs/image/common/* /usr/bin/
RUN chmod 0755 \
      /usr/bin/start.sh \
      /usr/bin/update-control-plane-ca \
      /usr/bin/pasturestack-nfs && \
    pasturestack-storage-runtime --help >/dev/null
CMD ["start.sh", "pasturestack-storage-runtime", "--driver-name", "pasturestack-nfs"]
