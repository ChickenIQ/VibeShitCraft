package world

import (
	"testing"
)

func TestFlatWorldBlock(t *testing.T) {
	tests := []struct {
		y    int32
		want uint16
	}{
		{-1, 0},          // below world: air
		{0, 7 << 4},      // bedrock
		{1, 3 << 4},      // dirt
		{2, 3 << 4},      // dirt
		{3, 3 << 4},      // dirt
		{4, 2 << 4},      // grass
		{5, 0},            // air
		{100, 0},          // air
		{256, 0},          // above world: air
	}

	for _, tt := range tests {
		got := FlatWorldBlock(tt.y)
		if got != tt.want {
			t.Errorf("FlatWorldBlock(%d) = %d, want %d", tt.y, got, tt.want)
		}
	}
}

func TestWorldGetSetBlock(t *testing.T) {
	w := NewWorld(42)

	// Unmodified: y=0 is always bedrock
	if got := w.GetBlock(0, 0, 0); got != 7<<4 {
		t.Errorf("GetBlock(0,0,0) = %d, want %d (bedrock)", got, 7<<4)
	}

	// Read a generated block, then override it and confirm
	original := w.GetBlock(5, 50, 5)
	w.SetBlock(5, 50, 5, 0)
	if got := w.GetBlock(5, 50, 5); got != 0 {
		t.Errorf("after SetBlock, GetBlock(5,50,5) = %d, want 0 (air)", got)
	}

	// Nearby unmodified position should still return generated value
	other := w.GetBlock(6, 50, 5)
	_ = original
	_ = other // just ensure no panic
}

func TestWorldGetModifications(t *testing.T) {
	w := NewWorld(42)

	// No modifications initially
	mods := w.GetModifications()
	if len(mods) != 0 {
		t.Errorf("expected 0 modifications, got %d", len(mods))
	}

	// Set some blocks
	w.SetBlock(1, 2, 3, 0)
	w.SetBlock(4, 5, 6, 3<<4)

	mods = w.GetModifications()
	if len(mods) != 2 {
		t.Errorf("expected 2 modifications, got %d", len(mods))
	}
	if mods[BlockPos{1, 2, 3}] != 0 {
		t.Errorf("modification at (1,2,3) = %d, want 0", mods[BlockPos{1, 2, 3}])
	}
	if mods[BlockPos{4, 5, 6}] != 3<<4 {
		t.Errorf("modification at (4,5,6) = %d, want %d", mods[BlockPos{4, 5, 6}], 3<<4)
	}
}

func TestBlockToItemID(t *testing.T) {
	tests := []struct {
		blockState uint16
		want       int16
	}{
		{0, -1},          // air -> no drop
		{7 << 4, -1},     // bedrock -> no drop
		{2 << 4, 3},      // grass -> dirt
		{1 << 4, 4},      // stone -> cobblestone
		{3 << 4, 3},      // dirt -> dirt
		{4 << 4, 4},      // cobblestone -> cobblestone
		{17 << 4, 17},    // oak log -> oak log
	}

	for _, tt := range tests {
		got := BlockToItemID(tt.blockState)
		if got != tt.want {
			t.Errorf("BlockToItemID(%d) = %d, want %d", tt.blockState, got, tt.want)
		}
	}
}
