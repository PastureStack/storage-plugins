package locales

import "embed"

// FS contains the complete, explicit runtime locale set.
//
//go:embed *.json
var FS embed.FS
