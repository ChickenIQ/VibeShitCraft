package world

import "sync"

// BlockPos represents a block position in the world.
type BlockPos struct {
	X, Y, Z int32
}

// ChunkPos represents a chunk position in the world.
type ChunkPos struct {
	X, Z int32
}

// Chunk represents a realized chunk column.
type Chunk struct {
	Sections [SectionsPerChunk][ChunkSectionSize]uint16
	Biomes   [256]byte
}

// World tracks the state of all blocks, including modifications and cached chunks.
type World struct {
	mu     sync.RWMutex
	blocks map[BlockPos]uint16 // manual block overrides (set by SetBlock)
	chunks map[ChunkPos]*Chunk // realized chunk cache
	Gen    *Generator
}

// NewWorld creates a new World with the given seed for terrain generation.
func NewWorld(seed int64) *World {
	return &World{
		blocks: make(map[BlockPos]uint16),
		chunks: make(map[ChunkPos]*Chunk),
		Gen:    NewGenerator(seed),
	}
}

// GetBlock returns the block state (blockID << 4 | metadata) at the given position.
func (w *World) GetBlock(x, y, z int32) uint16 {
	if y < 0 || y > 255 {
		return 0
	}

	w.mu.RLock()
	// Check overrides first
	if b, ok := w.blocks[BlockPos{x, y, z}]; ok {
		w.mu.RUnlock()
		return b
	}

	// Check chunk cache
	cp := ChunkPos{x >> 4, z >> 4}
	if chunk, ok := w.chunks[cp]; ok {
		w.mu.RUnlock()
		lx, ly, lz := x&0x0F, y&0x0F, z&0x0F
		sec := y >> 4
		return chunk.Sections[sec][(ly*16+lz)*16+lx]
	}
	w.mu.RUnlock()

	// Realize chunk
	w.mu.Lock()
	// Double-check after lock
	if chunk, ok := w.chunks[cp]; ok {
		w.mu.Unlock()
		lx, ly, lz := x&0x0F, y&0x0F, z&0x0F
		sec := y >> 4
		return chunk.Sections[sec][(ly*16+lz)*16+lx]
	}

	sections, biomes := w.Gen.GenerateInternal(int(cp.X), int(cp.Z))
	chunk := &Chunk{
		Sections: sections,
		Biomes:   biomes,
	}

	// Apply ALL existing overrides for this chunk to the realized chunk
	// This ensures consistency even if modifications happen before realization.
	for pos, state := range w.blocks {
		if pos.X>>4 == cp.X && pos.Z>>4 == cp.Z {
			lx, ly, lz := pos.X&0x0F, pos.Y&0x0F, pos.Z&0x0F
			sec := pos.Y >> 4
			chunk.Sections[sec][(ly*16+lz)*16+lx] = state
		}
	}

	w.chunks[cp] = chunk
	w.mu.Unlock()

	lx, ly, lz := x&0x0F, y&0x0F, z&0x0F
	sec := y >> 4
	return chunk.Sections[sec][(ly*16+lz)*16+lx]
}

// SetBlock sets the block state at the given position.
func (w *World) SetBlock(x, y, z int32, state uint16) {
	if y < 0 || y > 255 {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Record the modification
	w.blocks[BlockPos{x, y, z}] = state

	// Update cached chunk if it exists
	cp := ChunkPos{x >> 4, z >> 4}
	if chunk, ok := w.chunks[cp]; ok {
		lx, ly, lz := x&0x0F, y&0x0F, z&0x0F
		sec := y >> 4
		chunk.Sections[sec][(ly*16+lz)*16+lx] = state
	}
}

// GetChunkData returns the serialized chunk data for the given chunk coordinates.
// It uses cached chunks if available, otherwise it realizes them.
func (w *World) GetChunkData(cx, cz int32) ([]byte, uint16) {
	w.mu.RLock()
	cp := ChunkPos{cx, cz}
	if chunk, ok := w.chunks[cp]; ok {
		data, mask := SerializeSections(&chunk.Sections, chunk.Biomes)
		w.mu.RUnlock()
		return data, mask
	}
	w.mu.RUnlock()

	// Realize chunk
	w.mu.Lock()
	if chunk, ok := w.chunks[cp]; ok {
		data, mask := SerializeSections(&chunk.Sections, chunk.Biomes)
		w.mu.Unlock()
		return data, mask
	}

	sections, biomes := w.Gen.GenerateInternal(int(cx), int(cz))
	chunk := &Chunk{
		Sections: sections,
		Biomes:   biomes,
	}

	// Apply ALL existing overrides for this chunk to the realized chunk
	for pos, state := range w.blocks {
		if pos.X>>4 == cx && pos.Z>>4 == cz {
			lx, ly, lz := pos.X&0x0F, pos.Y&0x0F, pos.Z&0x0F
			sec := pos.Y >> 4
			chunk.Sections[sec][(ly*16+lz)*16+lx] = state
		}
	}

	w.chunks[cp] = chunk
	data, mask := SerializeSections(&chunk.Sections, chunk.Biomes)
	w.mu.Unlock()

	return data, mask
}

// GetModifications returns a copy of all modified blocks.
func (w *World) GetModifications() map[BlockPos]uint16 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make(map[BlockPos]uint16, len(w.blocks))
	for k, v := range w.blocks {
		result[k] = v
	}
	return result
}

// FlatWorldBlock returns the default block state for a flat world at the given Y level.
func FlatWorldBlock(y int32) uint16 {
	if y < 0 || y > 255 {
		return 0
	}
	switch {
	case y == 0:
		return 7 << 4 // bedrock
	case y <= 3:
		return 3 << 4 // dirt
	case y == 4:
		return 2 << 4 // grass
	default:
		return 0 // air
	}
}

// IsInstantBreak returns true for blocks with zero hardness that the client
// breaks with just a status=0 (start digging) packet in survival mode.
func IsInstantBreak(blockID uint16) bool {
	switch blockID {
	case 6, // sapling
		27, 28, // powered/detector rails
		30,     // cobweb (actually not instant but low)
		31, 32, // tall grass, dead bush
		37, 38, 39, 40, // flowers, mushrooms
		50,     // torch
		51,     // fire
		55,     // redstone wire
		59,     // wheat
		63, 68, // sign post, wall sign
		65,     // ladder
		66,     // rail
		69,     // lever
		70, 72, // stone/wooden pressure plate
		75, 76, // redstone torches
		77,     // button
		78,     // snow layer
		83,     // sugar cane
		90,     // nether portal
		93, 94, // repeater
		106,      // vine
		111,      // lily pad
		115,      // nether wart
		119, 120, // end portal, end portal frame
		131, 132, // tripwire hook, tripwire
		141, 142, // carrot, potato
		143,      // wooden button
		144,      // head
		147, 148, // golden/stone pressure plate
		149, 150, // comparator
		151,      // daylight sensor
		154,      // hopper
		157,      // activator rail
		175,      // double plant (tall grass, etc.)
		176, 177: // banner (standing/wall)
		return true
	}
	return false
}

// BlockToItemID returns the item ID, damage (metadata), and count that should be dropped when a block is broken.
// Returns -1 for itemID if the block should not drop anything.
func BlockToItemID(blockState uint16) (int16, int16, byte) {
	blockID := blockState >> 4
	metadata := int16(blockState & 0x0F)

	switch blockID {
	case 0: // air
		return -1, 0, 0
	case 7: // bedrock
		return -1, 0, 0
	case 8, 9, 10, 11: // water/lava
		return -1, 0, 0
	case 20, 95, 102, 160: // glass, stained glass, glass panes, stained glass panes
		return -1, 0, 0
	case 2: // grass block -> drops dirt
		return 3, 0, 1
	case 1: // stone -> meta 0 drops cobblestone, variants (granite, diorite, andesite) drop themselves
		if metadata == 0 {
			return 4, 0, 1
		}
		return 1, metadata, 1
	case 17, 162: // logs -> drop themselves, strip rotation/orientation
		// Log metadata: lowest 2 bits are wood type, next 2 are orientation
		return int16(blockID), metadata & 0x03, 1
	case 18, 161: // leaves -> chance to drop saplings, but for now drop nothing
		return -1, 0, 0
	case 31: // tall grass -> drops nothing (unless shears)
		return -1, 0, 0
	case 59: // wheat
		if metadata == 7 { // Fully grown
			return 296, 0, 1 // Wheat item
		}
		return 295, 0, 1 // Wheat seeds
	case 60: // farmland -> drops dirt
		return 3, 0, 1
	case 64: // oak door block
		return 324, 0, 1
	case 71: // iron door block
		return 330, 0, 1
	case 193: // spruce door block
		return 427, 0, 1
	case 194: // birch door block
		return 428, 0, 1
	case 195: // jungle door block
		return 429, 0, 1
	case 196: // acacia door block
		return 430, 0, 1
	case 197: // dark oak door block
		return 431, 0, 1
	case 175: // double plant
		if metadata&0x08 != 0 {
			return -1, 0, 0
		}
		return 175, metadata & 0x07, 1
	case 53, 67, 108, 109, 114, 128, 134, 135, 136, 156, 163, 164, 180: // stairs
		return int16(blockID), 0, 1 // Stairs item has no metadata for orientation
	case 50: // torch -> drop torch item with no metadata
		return 50, 0, 1
	case 75, 76: // redstone torch (off/on) -> drop redstone torch item
		return 76, 0, 1
	case 16: // coal ore -> coal
		return 263, 0, 1
	case 56: // diamond ore -> diamond
		return 264, 0, 1
	case 73, 74: // redstone ore -> redstone
		return 331, 0, 4 // roughly 4-5
	case 21: // lapis ore -> lapis dye (ID 351, meta 4)
		return 351, 4, 6 // roughly 4-8
	case 129: // emerald ore -> emerald
		return 388, 0, 1
	case 153: // quartz ore -> quartz
		return 406, 0, 1
	case 82: // clay block -> 4 clay balls
		return 337, 0, 4
	case 89: // glowstone -> 2-4 glowstone dust
		return 348, 0, 3
	case 169: // sea lantern -> 2-3 prismarine crystals
		return 410, 0, 2
	case 3: // dirt
		if metadata == 1 {
			return 3, 1, 1 // Coarse dirt drops itself
		}
		return 3, 0, 1 // Regular dirt and podzol drop regular dirt
	case 4: // cobblestone
		return 4, 0, 1
	case 5: // planks
		return 5, metadata, 1
	case 6: // sapling -> strip age bit, keep tree type
		return 6, metadata & 0x07, 1
	case 12: // sand
		return 12, metadata, 1
	case 13: // gravel
		return 13, 0, 1
	case 19: // sponge (dry/wet)
		return 19, metadata, 1
	case 24: // sandstone variants
		return 24, metadata, 1
	case 26: // bed -> only drop from foot part
		if metadata&0x08 != 0 {
			return -1, 0, 0
		}
		return 355, 0, 1
	case 35: // wool (16 colors)
		return 35, metadata, 1
	case 37: // dandelion
		return 37, 0, 1
	case 38: // flowers (poppy variants)
		return 38, metadata, 1
	case 43: // double stone slab -> drops 2 stone slabs
		return 44, metadata & 0x07, 2
	case 44: // stone slab -> strip upper/lower bit
		return 44, metadata & 0x07, 1
	case 54: // chest
		return 54, 0, 1
	case 61, 62: // furnace / lit furnace -> always drop furnace
		return 61, 0, 1
	case 97: // monster egg (silverfish blocks)
		return 97, metadata, 1
	case 98: // stone bricks variants
		return 98, metadata, 1
	case 125: // double wooden slab -> drops 2 wooden slabs
		return 126, metadata & 0x07, 2
	case 126: // wooden slab -> strip upper/lower bit
		return 126, metadata & 0x07, 1
	case 139: // cobblestone wall (cobblestone/mossy)
		return 139, metadata, 1
	case 145: // anvil -> strip rotation, keep damage level
		return 145, (metadata >> 2) & 0x03, 1
	case 155: // quartz block -> pillar orientations all drop as pillar
		if metadata >= 2 {
			return 155, 2, 1
		}
		return 155, metadata, 1
	case 159: // stained clay (16 colors)
		return 159, metadata, 1
	case 168: // prismarine variants
		return 168, metadata, 1
	case 171: // carpet (16 colors)
		return 171, metadata, 1
	case 179: // red sandstone variants
		return 179, metadata, 1
	default:
		return int16(blockID), 0, 1
	}
}
