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
}

// NewWorld creates a new World.
func NewWorld() *World {
	return &World{
		blocks: make(map[BlockPos]uint16),
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
	return FlatWorldBlock(y)
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

// BlockToItemID returns the item ID that should be dropped when a block is broken.
// Returns -1 if the block should not drop anything.
func BlockToItemID(blockState uint16) int16 {
	blockID := blockState >> 4
	switch blockID {
	case 0: // air
		return -1
	case 7: // bedrock
		return -1
	case 2: // grass block -> drops dirt
		return 3
	case 1: // stone -> drops cobblestone
		return 4
	default:
		return int16(blockID)
	}
}
