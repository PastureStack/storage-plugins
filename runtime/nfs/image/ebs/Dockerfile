ARG GO_IMAGE=golang:1.27.0-bookworm@sha256:484ef6066fa69acb059fdfeda7ba2b8f7391f2ef6abc6f9b8411e669ebd56466
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
      org.opencontainers.image.description="PastureStack Amazon EBS storage driver for the compatible control plane." \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="${IMAGE_VERSION}"
RUN apk add --no-cache \
        aws-cli=2.32.7-r0 \
        bash=5.3.3-r1 \
        ca-certificates=20260611-r0 \
        curl=8.20.0-r0 \
        e2fsprogs=1.47.3-r0 \
        jq=1.8.1-r0 \
        nvme-cli=2.16-r1 \
        util-linux=2.41.4-r0
COPY --from=runtime-build /out/pasturestack-storage-runtime /usr/bin/
COPY runtime/nfs/image/ebs/pasturestack-ebs runtime/nfs/image/common/* /usr/bin/
RUN chmod 0755 \
      /usr/bin/start.sh \
      /usr/bin/update-control-plane-ca \
      /usr/bin/pasturestack-ebs && \
    aws --version && \
    pasturestack-storage-runtime --help >/dev/null
CMD ["start.sh", "pasturestack-storage-runtime", "--driver-name", "pasturestack-ebs"]
