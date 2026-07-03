//go:build screenshot

package screenshot

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"testing"
)

// buildChunk assembles one PNG chunk (length + type + data + CRC) for test
// fixtures. It does not need a spec-correct CRC for StripPNGMetadata's own
// purposes (the function only reads length/type to walk the chunk stream),
// but computing a real CRC keeps the fixture indistinguishable from a real
// PNG chunk if ever fed to a stricter reader.
func buildChunk(ctype string, data []byte) []byte {
	buf := make([]byte, 0, 8+len(data)+4)
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(data))) //nolint:gosec // test fixture: len(data) is always small and non-negative (G115)
	buf = append(buf, length...)
	buf = append(buf, []byte(ctype)...)
	buf = append(buf, data...)

	crc := crc32.ChecksumIEEE(append([]byte(ctype), data...))
	crcBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(crcBytes, crc)
	buf = append(buf, crcBytes...)
	return buf
}

// fixturePNG builds a minimal, structurally-valid (chunk-wise) in-memory PNG
// with an IHDR, an optional tIME chunk, an optional tEXt chunk, an IDAT, and
// an IEND — enough for StripPNGMetadata to exercise real chunk-walking logic
// without needing a real freeze/go-rod render.
func fixturePNG(includeTimestampMetadata bool) []byte {
	var buf bytes.Buffer
	buf.Write(pngSignature)
	buf.Write(buildChunk("IHDR", make([]byte, 13)))
	if includeTimestampMetadata {
		buf.Write(buildChunk("tIME", []byte{0x07, 0xE8, 1, 1, 0, 0, 0}))
		buf.Write(buildChunk("tEXt", []byte("Software\x00freeze v0.2.2")))
	}
	buf.Write(buildChunk("IDAT", []byte{1, 2, 3, 4, 5}))
	buf.Write(buildChunk("IEND", nil))
	return buf.Bytes()
}

func TestStripPNGMetadata_RemovesTimeAndTextChunks(t *testing.T) {
	png := fixturePNG(true)

	stripped, err := StripPNGMetadata(png)
	if err != nil {
		t.Fatalf("StripPNGMetadata: unexpected error: %v", err)
	}
	if bytes.Contains(stripped, []byte("tIME")) {
		t.Error("StripPNGMetadata: tIME chunk was not removed")
	}
	if bytes.Contains(stripped, []byte("tEXt")) {
		t.Error("StripPNGMetadata: tEXt chunk was not removed")
	}
	if !bytes.Contains(stripped, []byte("IDAT")) {
		t.Error("StripPNGMetadata: IDAT chunk was incorrectly removed")
	}
	if !bytes.Contains(stripped, []byte("IEND")) {
		t.Error("StripPNGMetadata: IEND chunk was incorrectly removed")
	}
}

func TestStripPNGMetadata_Idempotent(t *testing.T) {
	png := fixturePNG(true)

	first, err := StripPNGMetadata(png)
	if err != nil {
		t.Fatalf("StripPNGMetadata (1st pass): unexpected error: %v", err)
	}
	second, err := StripPNGMetadata(first)
	if err != nil {
		t.Fatalf("StripPNGMetadata (2nd pass): unexpected error: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Error("StripPNGMetadata is not idempotent: stripping an already-stripped PNG changed its bytes")
	}
}

func TestStripPNGMetadata_RejectsBadSignature(t *testing.T) {
	if _, err := StripPNGMetadata([]byte("not a png")); err == nil {
		t.Error("StripPNGMetadata: expected an error for a non-PNG input, got nil")
	}
}

func TestHashPNG_StableForFixedInput(t *testing.T) {
	png := fixturePNG(true)

	h1, err := HashPNG(png)
	if err != nil {
		t.Fatalf("HashPNG (1st call): unexpected error: %v", err)
	}
	h2, err := HashPNG(png)
	if err != nil {
		t.Fatalf("HashPNG (2nd call): unexpected error: %v", err)
	}
	if h1 != h2 {
		t.Errorf("HashPNG is not stable for a fixed input: %q != %q", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("HashPNG: expected a 64-char hex SHA-256, got %d chars", len(h1))
	}
}

func TestHashPNG_UnaffectedByTimestampMetadata(t *testing.T) {
	withoutMetadata := fixturePNG(false)
	withMetadata := fixturePNG(true)

	h1, err := HashPNG(withoutMetadata)
	if err != nil {
		t.Fatalf("HashPNG (without metadata): unexpected error: %v", err)
	}
	h2, err := HashPNG(withMetadata)
	if err != nil {
		t.Fatalf("HashPNG (with metadata): unexpected error: %v", err)
	}
	if h1 != h2 {
		t.Errorf("HashPNG: golden hash must be identical regardless of tIME/tEXt chunk presence, got %q vs %q", h1, h2)
	}
}
