package main

import (
	"github.com/castocolina/gitid/internal/upload"
)

// uploadInstructions returns the provider-specific steps for uploading a
// freshly generated public key (UP-01/UP-02). It delegates to internal/upload
// so that both cmd/gitid and tui can access the same instruction strings
// without duplication.
//
// See internal/upload for the full provider-specific documentation.
func uploadInstructions(provider string) string {
	return upload.Instructions(provider)
}
