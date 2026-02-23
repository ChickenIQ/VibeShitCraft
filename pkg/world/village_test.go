package world

import (
	"testing"
)

func TestDivFloor(t *testing.T) {
	tests := []struct {
		a, b int
		want int
	}{
		{10, 3, 3},
		{-10, 3, -4},
		{16, 16, 1},
		{-16, 16, -1},
		{0, 16, 0},
		{-1, 16, -1},
		{-17, 16, -2},
	}
	for _, tt := range tests {
		if got := divFloor(tt.a, tt.b); got != tt.want {
			t.Errorf("divFloor(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestVillageStructureContinuity(t *testing.T) {
	// Pick a seed and a village center that spans chunk boundaries.
	// Seed 42, Cell (0,0) center is (10, 10).
	// A 7x9 Hall at (10, 32) will span chunks (0,2) and (1,2) if it overlaps X=16.
	// We want to ensure that generating chunk (0,2) and (1,2) independently 
	// results in a continuous structure.
	
	g1 := NewGenerator(42)
	
	// Coordinates of a block in the hall that should be generated in both chunks
	// if we were to generate the whole structure into a large buffer.
	// Let's check a specific block in the Large Hall.
	// Hall at vx, vz+22. Hall width is 7. VX=10, VZ=10.
	// Hall X range: [10, 16]. It ends exactly at the boundary of chunk 0 and 1.
	// If it was hx+dx where dx goes to 6, hx=10, 10+6=16.
	// So world X=16 is the first block of chunk 1.
	
	// Let's generate chunk 0 and chunk 1 around world Z=32 (Hall Z).
	_, mask0 := g1.GenerateChunkData(0, 2) // world Z [32, 47]
	_, mask1 := g1.GenerateChunkData(1, 2) // world Z [32, 47]
	
	// If the hall is present, these masks should be non-zero for the surface sections.
	if mask0 == 0 || mask1 == 0 {
		t.Errorf("Village structures missing at boundaries: mask0=0x%x, mask1=0x%x", mask0, mask1)
	}
}
