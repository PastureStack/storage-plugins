# Origin and distribution boundary

The migration baseline is the historical github.com/rancher/storage source at commit b8c008dec37cf1e0730366a837ddd415481a7f93, tagged v0.9.11, with 180 commits and 46 tags in the audited local history.
That project was published by Rancher Labs under the Apache License, Version 2.0; the inherited root LICENSE is retained byte for byte.

The first PastureStack maintenance commit removed 2,788 tracked paths belonging
to former runtime adapters, build and packaging automation, vendored
dependencies, and embedded secret-delivery snapshots. None of those sources is
imported, compiled, linked, or distributed by the offline
`storage-plugins` planner.

The NFS-only runtime behavior was subsequently restored from the audited
v0.9.11 working tree as a reviewed maintenance change. Obsolete Docker
engine-api, Kubernetes monolith, and control-plane SDK dependencies were then
replaced by maintained modules and a small same-origin client limited to the
required schemas and actions. Current third-party source and license files are
recorded under `runtime/nfs/vendor`; the image, executable, driver,
documentation, and maintained identifiers use the PastureStack namespace.
Block, cloud, and secret-delivery runtime snapshots were not restored.

The planner remains an offline, standard-library implementation. The privileged
NFS adapter is a separate Docker build target and is not imported by the
planner. Required legacy protocol literals and their narrow purpose are listed
in [COMPATIBILITY.md](COMPATIBILITY.md). Go toolchain legal notices are recorded
separately in [`LICENSES/`](LICENSES/).
