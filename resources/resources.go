package resources

import "embed"

// FS holds the embedded web resources (HTML, CSS, JS, images)
//
//go:embed *
var FS embed.FS
