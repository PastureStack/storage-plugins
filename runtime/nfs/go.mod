module github.com/PastureStack/storage-plugins/runtime/nfs

go 1.26.0

// The release image intentionally builds this compatibility runtime in
// GOPATH/vendor mode. This module boundary keeps the offline planner's
// standard-library-only test surface independent from the legacy adapter.
