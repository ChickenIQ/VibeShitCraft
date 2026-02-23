package world

import (
	"bytes"
	"encoding/binary"
)

const (
	ChunkSectionSize = 16 * 16 * 16
	ChunkHeight      = 256
	SectionsPerChunk = ChunkHeight / 16
)

// GenerateFlatChunkData generates chunk column data for a superflat world.
// Layers: 0=bedrock, 1-3=dirt, 4=grass, 5+=air
// This produces the data portion of the 1.8 Chunk Data packet (0x21).
func GenerateFlatChunkData() ([]byte, uint16) {
	var primaryBitMask uint16

	// We need sections 0 (y=0..15) - contains all solid blocks
	primaryBitMask = 0x0001

	var buf bytes.Buffer

	// Section 0: blocks at y=0..15
	// In 1.8, each block is 2 bytes: (blockID << 4) | metadata
	// Block IDs: bedrock=7, dirt=3, grass=2, air=0
	blockData := make([]uint16, ChunkSectionSize)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := 0; y < 16; y++ {
				idx := ((y * 16) + z) * 16 + x
				switch {
				case y == 0:
					blockData[idx] = 7 << 4 // bedrock
				case y <= 3:
					blockData[idx] = 3 << 4 // dirt
				case y == 4:
					blockData[idx] = 2 << 4 // grass
				default:
					blockData[idx] = 0 // air
				}
			}
		}
	}

	// Write block data (2 bytes per block, little endian)
	for _, b := range blockData {
		binary.Write(&buf, binary.LittleEndian, b)
	}

	// Block light (half byte per block = 2048 bytes), all 0xFF (full light)
	blockLight := make([]byte, 2048)
	for i := range blockLight {
		blockLight[i] = 0xFF
	}
	buf.Write(blockLight)

	// Sky light (half byte per block = 2048 bytes), all 0xFF
	skyLight := make([]byte, 2048)
	for i := range skyLight {
		skyLight[i] = 0xFF
	}
	buf.Write(skyLight)

	// Biome data (256 bytes) - only sent when ground-up continuous is true
	// Use plains biome (1)
	biomes := make([]byte, 256)
	for i := range biomes {
		biomes[i] = 1 // plains
	}
	buf.Write(biomes)

	return buf.Bytes(), primaryBitMask
}
