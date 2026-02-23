package world

import (
	"testing"
)

func TestGenerateFlatChunkData(t *testing.T) {
	data, bitmask := GenerateFlatChunkData()

	// Bitmask should indicate section 0 only
	if bitmask != 0x0001 {
		t.Errorf("primaryBitMask = 0x%04x, want 0x0001", bitmask)
	}

	// Data should not be empty
	if len(data) == 0 {
		t.Error("chunk data is empty")
	}

	// Expected size: 8192 (block data) + 2048 (block light) + 2048 (sky light) + 256 (biomes) = 12544
	expectedSize := 16*16*16*2 + 2048 + 2048 + 256
	if len(data) != expectedSize {
		t.Errorf("chunk data size = %d, want %d", len(data), expectedSize)
	}
}
