module github.com/PastureStack/storage-plugins/runtime/nfs

go 1.27

require (
	github.com/docker/go-plugins-helpers v0.0.0-20240701071450-45e2431495c8
	github.com/moby/moby/api v1.55.0
	github.com/moby/moby/client v0.5.1
	github.com/sirupsen/logrus v1.10.1
	github.com/urfave/cli/v3 v3.11.0
	k8s.io/mount-utils v0.36.4
	k8s.io/utils v0.0.0-20260707023825-cf1189d6abe3
)

// Docker's current plugin helper still imports the pre-module CoreOS path.
// Bind that compatible import to the maintained v22 implementation.
replace github.com/coreos/go-systemd => github.com/coreos/go-systemd/v22 v22.7.0

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/go-connections v0.8.1 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.70.0 // indirect
	go.opentelemetry.io/otel v1.45.0 // indirect
	go.opentelemetry.io/otel/metric v1.45.0 // indirect
	go.opentelemetry.io/otel/trace v1.45.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
)
