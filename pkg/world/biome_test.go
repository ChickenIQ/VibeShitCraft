package world

import "testing"

func TestBiomeAtDeterminism(t *testing.T) {
	temp := NewPerlin(100)
	rain := NewPerlin(200)

	for i := 0; i < 50; i++ {
		x := i*31 - 500
		z := i*17 - 300
		b1 := BiomeAt(temp, rain, x, z)
		b2 := BiomeAt(temp, rain, x, z)
		if b1.ID != b2.ID {
			t.Errorf("BiomeAt(%d,%d) not deterministic: %s vs %s", x, z, b1.Name, b2.Name)
		}
	}
}

func TestAllBiomesReachable(t *testing.T) {
	temp := NewPerlin(42)
	rain := NewPerlin(43)

	found := make(map[byte]bool)
	// Sweep a large area to find multiple biomes
	for x := -500; x < 500; x += 7 {
		for z := -500; z < 500; z += 7 {
			b := BiomeAt(temp, rain, x, z)
			found[b.ID] = true
		}
	}

	// We should find at least 4 distinct biomes in a 1000x1000 area
	if len(found) < 4 {
		t.Errorf("only found %d distinct biomes in 1000x1000 area, want >= 4: %v", len(found), found)
	}
}

func TestBiomeFieldsValid(t *testing.T) {
	for _, b := range allBiomes {
		if b.Name == "" {
			t.Errorf("biome ID %d has empty name", b.ID)
		}
		if b.BaseHeight < 1 || b.BaseHeight > 255 {
			t.Errorf("biome %s has invalid BaseHeight: %d", b.Name, b.BaseHeight)
		}
		if b.HeightVariation < 0 {
			t.Errorf("biome %s has negative HeightVariation: %f", b.Name, b.HeightVariation)
		}
	}
}
