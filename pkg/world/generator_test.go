package world

import "testing"

func TestGeneratorDeterminism(t *testing.T) {
	g1 := NewGenerator(12345)
	g2 := NewGenerator(12345)

	data1, mask1 := g1.GenerateChunkData(0, 0)
	data2, mask2 := g2.GenerateChunkData(0, 0)

	if mask1 != mask2 {
		t.Errorf("bitmask mismatch: 0x%04x vs 0x%04x", mask1, mask2)
	}
	if len(data1) != len(data2) {
		t.Fatalf("data length mismatch: %d vs %d", len(data1), len(data2))
	}
	for i := range data1 {
		if data1[i] != data2[i] {
			t.Fatalf("data differs at byte %d", i)
		}
	}
}

func TestChunkDataNotEmpty(t *testing.T) {
	g := NewGenerator(42)
	data, mask := g.GenerateChunkData(0, 0)

	if mask == 0 {
		t.Error("primaryBitMask is 0, expected at least one section")
	}
	if len(data) == 0 {
		t.Error("chunk data is empty")
	}
}

func TestBedrockLayer(t *testing.T) {
	g := NewGenerator(999)

	// y=0 should always be bedrock regardless of x, z
	for x := -100; x < 100; x += 17 {
		for z := -100; z < 100; z += 17 {
			block := g.BlockAt(x, 0, z)
			if block != 7<<4 {
				t.Errorf("BlockAt(%d, 0, %d) = %d, want %d (bedrock)", x, z, block, 7<<4)
			}
		}
	}
}

func TestSurfaceHeightRange(t *testing.T) {
	g := NewGenerator(555)

	for x := -200; x < 200; x += 13 {
		for z := -200; z < 200; z += 13 {
			h := g.SurfaceHeight(x, z)
			if h < 1 || h > 250 {
				t.Errorf("SurfaceHeight(%d, %d) = %d, out of valid range [1, 250]", x, z, h)
			}
		}
	}
}

func TestBlockAtBoundary(t *testing.T) {
	g := NewGenerator(42)

	// Below world = air
	if got := g.BlockAt(0, -1, 0); got != 0 {
		t.Errorf("BlockAt(0, -1, 0) = %d, want 0 (air)", got)
	}
	// Above world = air
	if got := g.BlockAt(0, 256, 0); got != 0 {
		t.Errorf("BlockAt(0, 256, 0) = %d, want 0 (air)", got)
	}
}

func TestDifferentChunksVary(t *testing.T) {
	g := NewGenerator(42)

	data1, _ := g.GenerateChunkData(0, 0)
	data2, _ := g.GenerateChunkData(10, 10)

	if len(data1) == len(data2) {
		same := true
		for i := range data1 {
			if data1[i] != data2[i] {
				same = false
				break
			}
		}
		if same {
			t.Error("distant chunks produced identical data — terrain not varying")
		}
	}
}
