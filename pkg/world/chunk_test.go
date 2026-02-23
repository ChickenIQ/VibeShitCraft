package world

import (
	"encoding/binary"
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

func TestSerializeSectionsNonInterleaved(t *testing.T) {
	// Verify that SerializeSections writes data in the correct 1.8 protocol order:
	// 1. All block data for all sections
	// 2. All block light for all sections
	// 3. All sky light for all sections
	// 4. Biome data
	// This prevents the 8-block gap that occurs when data is interleaved per section.
	var sections [SectionsPerChunk][ChunkSectionSize]uint16
	var biomes [256]byte

	// Place a distinguishable block in section 0 (y=0) and section 1 (y=16)
	// Section 0: place bedrock (7<<4 = 0x0070) at (0,0,0)
	sections[0][0] = 7 << 4
	// Section 1: place stone (1<<4 = 0x0010) at (0,0,0) which is y=16 in world
	sections[1][0] = 1 << 4

	data, mask := SerializeSections(&sections, biomes)

	// Both sections should be marked as active
	if mask != 0x0003 {
		t.Fatalf("primaryBitMask = 0x%04x, want 0x0003", mask)
	}

	const blockDataPerSection = ChunkSectionSize * 2 // 4096 * 2 = 8192 bytes
	const lightPerSection = 2048
	numSections := 2
	expectedSize := numSections*blockDataPerSection + numSections*lightPerSection + numSections*lightPerSection + 256
	if len(data) != expectedSize {
		t.Fatalf("data length = %d, want %d", len(data), expectedSize)
	}

	// In non-interleaved format:
	// - Section 0 block data starts at offset 0
	// - Section 1 block data starts at offset 8192
	// - Block light starts at offset 8192*2 = 16384
	// Verify that section 1 block data starts immediately after section 0 block data
	// (NOT after section 0's light data, which would be the interleaved bug)
	sec1BlockStart := blockDataPerSection // offset 8192
	// First uint16 of section 1 should be stone (1<<4 = 0x0010)
	sec1FirstBlock := binary.LittleEndian.Uint16(data[sec1BlockStart:])
	if sec1FirstBlock != 1<<4 {
		t.Errorf("section 1 first block at offset %d = 0x%04x, want 0x%04x (stone); data may be interleaved instead of sequential",
			sec1BlockStart, sec1FirstBlock, uint16(1<<4))
	}

	// Verify that block light starts after ALL block data
	lightStart := numSections * blockDataPerSection // 16384
	// Block light should be 0xFF (full bright)
	if data[lightStart] != 0xFF {
		t.Errorf("block light at offset %d = 0x%02x, want 0xFF; block light may be misplaced",
			lightStart, data[lightStart])
	}
}
