# Origin and distribution boundary

The migration baseline is the historical github.com/rancher/storage source at commit b8c008dec37cf1e0730366a837ddd415481a7f93, tagged v0.9.11, with 180 commits and 46 tags in the audited local history.
That project was published by Rancher Labs under the Apache License, Version 2.0; the inherited root LICENSE is retained byte for byte.

The migration removed 2,788 tracked paths belonging to the former runtime adapters, build and packaging automation, vendored dependencies, and embedded secret-delivery snapshots. None of those deleted sources are imported, compiled, linked, or distributed by the new `storage-plugins` planner.

The deleted snapshot-reference files and later image version labels did not agree. `storage-plugins` therefore makes no release-version or provenance-equivalence claim for those snapshots. Secret delivery is delegated only to the repositories named in [COMPATIBILITY.md](COMPATIBILITY.md).

The new planner is an offline, standard-library implementation. It preserves storage capability and lifecycle intent while deliberately excluding runtime adapters, vendored runtime dependencies, credentials, sockets, host paths, network clients, and destructive execution. Go toolchain legal notices are recorded separately in [`LICENSES/`](LICENSES/).
