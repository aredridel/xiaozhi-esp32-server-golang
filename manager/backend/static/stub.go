//go:build !embed_ui

package static

import "embed"

// FS is empty when embed_ui is not enabled, frontend static resources are not mounted during development
var FS = embed.FS{}
