package world

// Biome describes terrain generation parameters for a biome.
type Biome struct {
	ID              byte    // Minecraft biome ID
	Name            string
	SurfaceBlock    uint16  // block state (blockID << 4 | meta)
	FillerBlock     uint16  // block below surface
	BaseHeight      int     // base terrain height in blocks
	HeightVariation float64 // amplitude of height noise
	TreeDensity     float64 // 0.0 = none, higher = more trees
	HasSnow         bool
}

// Predefined biomes
var (
	BiomeOcean = &Biome{
		ID: 0, Name: "Ocean",
		SurfaceBlock: 12 << 4, // sand
		FillerBlock:  12 << 4, // sand
		BaseHeight:   38, HeightVariation: 8,
		TreeDensity: 0,
	}
	BiomePlains = &Biome{
		ID: 1, Name: "Plains",
		SurfaceBlock: 2 << 4, // grass
		FillerBlock:  3 << 4, // dirt
		BaseHeight:   66, HeightVariation: 12,
		TreeDensity: 0.003,
	}
	BiomeDesert = &Biome{
		ID: 2, Name: "Desert",
		SurfaceBlock: 12 << 4, // sand
		FillerBlock:  24 << 4, // sandstone
		BaseHeight:   64, HeightVariation: 10,
		TreeDensity: 0,
	}
	BiomeExtremeHills = &Biome{
		ID: 3, Name: "Extreme Hills",
		SurfaceBlock: 2 << 4, // grass
		FillerBlock:  1 << 4, // stone
		BaseHeight:   72, HeightVariation: 50,
		TreeDensity: 0.004,
	}
	BiomeForest = &Biome{
		ID: 4, Name: "Forest",
		SurfaceBlock: 2 << 4, // grass
		FillerBlock:  3 << 4, // dirt
		BaseHeight:   68, HeightVariation: 14,
		TreeDensity: 0.025,
	}
	BiomeSnowyTundra = &Biome{
		ID: 12, Name: "Snowy Tundra",
		SurfaceBlock: 80 << 4, // snow block
		FillerBlock:  3 << 4,  // dirt
		BaseHeight:   66, HeightVariation: 8,
		TreeDensity: 0, HasSnow: true,
	}
)

// allBiomes is an ordered list used for selection lookups.
var allBiomes = []*Biome{
	BiomeOcean,
	BiomePlains,
	BiomeDesert,
	BiomeExtremeHills,
	BiomeForest,
	BiomeSnowyTundra,
}

// BiomeAt selects a biome for a world block position using temperature and
// rainfall noise values. The noise generators should use low-frequency scales
// so biomes form large regions.
func BiomeAt(tempNoise, rainNoise *Perlin, worldX, worldZ int) *Biome {
	// Low-frequency coordinates for large biome regions
	const scale = 0.003
	bx := float64(worldX) * scale
	bz := float64(worldZ) * scale

	temp := tempNoise.OctaveNoise2D(bx, bz, 4, 2.0, 0.5)  // −1..1
	rain := rainNoise.OctaveNoise2D(bx+500, bz+500, 4, 2.0, 0.5)

	// Map to 0..1
	temp = (temp + 1) / 2
	rain = (rain + 1) / 2

	// Selection based on temperature & rainfall
	switch {
	case temp < 0.25:
		return BiomeSnowyTundra
	case temp < 0.45:
		if rain > 0.5 {
			return BiomeForest
		}
		return BiomePlains
	case temp < 0.7:
		if rain > 0.6 {
			return BiomeForest
		}
		if rain < 0.25 {
			return BiomeExtremeHills
		}
		return BiomePlains
	default:
		if rain < 0.4 {
			return BiomeDesert
		}
		return BiomePlains
	}
}
