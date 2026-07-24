ARG GO_IMAGE=golang:1.26.5-bookworm
ARG RUNTIME_IMAGE=ubuntu:26.04
ARG IMAGE_VERSION=v0.9.13

FROM ${GO_IMAGE} AS runtime-build
ARG IMAGE_VERSION
WORKDIR /go/src/github.com/PastureStack/storage-plugins
COPY runtime/nfs ./runtime/nfs
ENV CGO_ENABLED=0 \
    GO111MODULE=off
RUN mkdir -p /out && \
    go build \
      -trimpath \
      -ldflags="-s -w -X main.VERSION=${IMAGE_VERSION}" \
      -o /out/pasturestack-storage-runtime \
      ./runtime/nfs

FROM ${RUNTIME_IMAGE}
ARG IMAGE_VERSION
ARG UBUNTU_MIRROR=http://archive.ubuntu.com/ubuntu
LABEL org.opencontainers.image.source="https://github.com/PastureStack/storage-plugins" \
      org.opencontainers.image.description="PastureStack NFS storage driver for the compatible control plane." \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="${IMAGE_VERSION}"
RUN set -eux; \
    rm -f /usr/bin/pebble; \
    find /etc/apt -type f \( -name '*.list' -o -name '*.sources' \) -exec sed -i \
        -e "s|http://security.ubuntu.com/ubuntu|${UBUNTU_MIRROR}|g" \
        -e "s|http://archive.ubuntu.com/ubuntu|${UBUNTU_MIRROR}|g" {} +; \
    printf 'Acquire::Retries "5";\nAcquire::http::Timeout "30";\nAcquire::https::Timeout "30";\nAcquire::http::Pipeline-Depth "0";\n' > /etc/apt/apt.conf.d/80pasturestack-retries; \
    apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        jq \
        netbase \
        nfs-common \
        rpcbind \
        util-linux && \
    rm -rf /var/lib/apt/lists/*
COPY --from=runtime-build /out/pasturestack-storage-runtime /usr/bin/
COPY runtime/nfs/image/nfs/pasturestack-nfs runtime/nfs/image/common/* /usr/bin/
RUN chmod 0755 \
      /usr/bin/start.sh \
      /usr/bin/update-control-plane-ca \
      /usr/bin/pasturestack-nfs
CMD ["start.sh", "pasturestack-storage-runtime", "--driver-name", "pasturestack-nfs"]
