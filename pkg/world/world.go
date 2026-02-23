package world

import "sync"

// BlockPos represents a block position in the world.
type BlockPos struct {
	X, Y, Z int32
}

// World tracks the state of all blocks, including modifications.
type World struct {
	mu     sync.RWMutex
	blocks map[BlockPos]uint16 // block state: blockID << 4 | metadata
	Gen    *Generator
}

// NewWorld creates a new World with the given seed for terrain generation.
func NewWorld(seed int64) *World {
	return &World{
		blocks: make(map[BlockPos]uint16),
		Gen:    NewGenerator(seed),
	}
}

// GetBlock returns the block state (blockID << 4 | metadata) at the given position.
func (w *World) GetBlock(x, y, z int32) uint16 {
	w.mu.RLock()
	if b, ok := w.blocks[BlockPos{x, y, z}]; ok {
		w.mu.RUnlock()
		return b
	}
	w.mu.RUnlock()
	return w.Gen.BlockAt(int(x), int(y), int(z))
}

// SetBlock sets the block state at the given position.
func (w *World) SetBlock(x, y, z int32, state uint16) {
	w.mu.Lock()
	w.blocks[BlockPos{x, y, z}] = state
	w.mu.Unlock()
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

// BlockToItemID returns the item ID and damage (metadata) that should be dropped when a block is broken.
// Returns -1 for itemID if the block should not drop anything.
func BlockToItemID(blockState uint16) (int16, int16) {
	blockID := blockState >> 4
	metadata := int16(blockState & 0x0F)

	switch blockID {
	case 0: // air
		return -1, 0
	case 7: // bedrock
		return -1, 0
	case 8, 9, 10, 11: // water/lava
		return -1, 0
	case 20, 95, 102: // glass, glass panes
		return -1, 0
	case 2: // grass block -> drops dirt
		return 3, 0
	case 1: // stone -> drops cobblestone
		return 4, 0
	case 17, 162: // logs -> drop themselves
		return int16(blockID), metadata
	case 18, 161: // leaves -> chance to drop saplings, but for now drop nothing
		return -1, 0
	case 31: // tall grass -> drops nothing (unless shears)
		return -1, 0
	case 59: // wheat -> drops wheat item (ID 296)
		return 296, 0
	case 60: // farmland -> drops dirt
		return 3, 0
	case 64: // wooden door block -> drops oak door item (ID 324)
		return 324, 0
	default:
		return int16(blockID), metadata
	}
}
