# PastureStack NFS storage driver

This directory contains the compatibility runtime for the
`pasturestack-nfs` Docker volume driver. It is built and released separately
from the offline planner in the repository root.

## Supported use cases

The driver supports NFS version 3 and 4 in two ownership modes:

1. A direct `host` and `export` reference is mounted as an externally managed
   share. The driver never creates or deletes data in this mode.
2. A default export base, or an explicit `host` and `exportBase`, owns one
   validated subdirectory per volume. Creation may create that subdirectory;
   removal retains it by default.

`purge` is opt-in and is restricted to the validated volume subdirectory. It
cannot recursively clear a direct export.

## Build

Run from the repository root:

```sh
docker build \
  --build-arg IMAGE_VERSION=v0.9.13 \
  --file runtime/nfs/image/nfs/Dockerfile \
  --tag nfs-storage-driver:v0.9.13 \
  .
```

The release image uses Ubuntu 26.04 and contains an amd64-compatible static Go
runtime plus the NFS client utilities.

## Runtime contract

The Catalog deployment supplies:

- `NFS_SERVER`: default server address;
- `MOUNT_DIR`: absolute export base;
- `MOUNT_OPTS`: comma-delimited NFS mount options;
- `ON_REMOVE`: `retain` by default, or explicit `purge`; and
- `PASTURESTACK_DEBUG`: optional shell tracing.

The service must run privileged and globally because it registers a Docker
volume socket and performs host-visible mounts. Required inherited API
environment variables and host paths are documented in the repository-level
[COMPATIBILITY.md](../../COMPATIBILITY.md).

## Release-reference policy

Catalog and Compose files use
`ghcr.io/pasturestack/nfs-storage-driver:<semantic-version>`. Digests are
recorded only in private release evidence and never appended to an operational
image reference.
