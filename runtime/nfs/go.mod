module github.com/PastureStack/storage-plugins/runtime/nfs

go 1.27.0

require golang.org/x/sys v0.47.0 // indirect

// The release image intentionally builds this compatibility runtime in
// GOPATH/vendor mode. The requirement above records the exact maintained
// x/sys source copied into vendor; it is not fetched during the image build.
// This module boundary keeps the offline planner's standard-library-only test
// surface independent from the compatibility adapter.
