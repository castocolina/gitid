//go:build screenshot

package screenshot

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// pngSignature is the fixed 8-byte magic every valid PNG file starts with.
var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// nonDeterministicChunks are PNG ancillary chunk types that may embed
// wall-clock timestamps or free-text metadata (creation time, tool
// signatures) that would otherwise make two visually-identical renders hash
// differently across machines/runs. StripPNGMetadata removes all of them so
// the golden hash reflects only the visual pixel content.
var nonDeterministicChunks = map[string]bool{
	"tIME": true, // image last-modification time
	"tEXt": true, // uncompressed textual metadata (e.g. tool name/version)
	"zTXt": true, // compressed textual metadata
	"iTXt": true, // international textual metadata
}

// StripPNGMetadata returns a copy of a PNG file's bytes with every
// non-deterministic ancillary chunk (tIME, tEXt, zTXt, iTXt) removed. It is
// idempotent: stripping an already-stripped PNG returns byte-identical
// output (determinism_test.go asserts this).
func StripPNGMetadata(png []byte) ([]byte, error) {
	if len(png) < 8 || !bytes.Equal(png[:8], pngSignature) {
		return nil, fmt.Errorf("screenshot: StripPNGMetadata: not a PNG file (bad signature)")
	}

	out := make([]byte, 0, len(png))
	out = append(out, png[:8]...)

	i := 8
	for i+8 <= len(png) {
		length := binary.BigEndian.Uint32(png[i : i+4])
		ctype := string(png[i+4 : i+8])
		chunkEnd := i + 8 + int(length) + 4 // length(4) + type(4) + data(length) + CRC(4)
		if chunkEnd > len(png) {
			return nil, fmt.Errorf("screenshot: StripPNGMetadata: malformed chunk %q: truncated", ctype)
		}
		if !nonDeterministicChunks[ctype] {
			out = append(out, png[i:chunkEnd]...)
		}
		i = chunkEnd
	}
	if i != len(png) {
		return nil, fmt.Errorf("screenshot: StripPNGMetadata: trailing bytes after last chunk")
	}
	return out, nil
}

// HashPNG returns the lowercase hex SHA-256 of PNG bytes, after first
// stripping non-deterministic metadata via StripPNGMetadata. This is the
// "golden hash" recorded in GOLDENS.md and compared on every capture re-run
// (TestCaptureTUI / TestCaptureHTML).
func HashPNG(png []byte) (string, error) {
	stripped, err := StripPNGMetadata(png)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(stripped)
	return hex.EncodeToString(sum[:]), nil
}
