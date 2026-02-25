package world

// Biome describes terrain generation parameters for a biome.
type Biome struct {
	ID              byte // Minecraft biome ID
	Name            string
	SurfaceBlock    uint16  // block state (blockID << 4 | meta)
	FillerBlock     uint16  // block below surface
	BaseHeight      int     // base terrain height in blocks
	HeightVariation float64 // amplitude of height noise
	TreeDensity     float64 // 0.0 = none, higher = more trees
	BoulderDensity  float64 // 0.0 = none, chance per column
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
		TreeDensity:    0.006,
		BoulderDensity: 0.03,
	}
	BiomeDesert = &Biome{
		ID: 2, Name: "Desert",
		SurfaceBlock: 12 << 4, // sand
		FillerBlock:  24 << 4, // sandstone
		BaseHeight:   64, HeightVariation: 10,
		TreeDensity:    0,
		BoulderDensity: 0.02, // desert rocks
	}
	BiomeExtremeHills = &Biome{
		ID: 3, Name: "Extreme Hills",
		SurfaceBlock: 2 << 4, // grass
		FillerBlock:  1 << 4, // stone
		BaseHeight:   72, HeightVariation: 50,
		TreeDensity:    0.015,
		BoulderDensity: 0.08,
	}
	BiomeForest = &Biome{
		ID: 4, Name: "Forest",
		SurfaceBlock: 2 << 4, // grass
		FillerBlock:  3 << 4, // dirt
		BaseHeight:   68, HeightVariation: 14,
		TreeDensity:    0.05,
		BoulderDensity: 0.04,
	}
	BiomeJungle = &Biome{
		ID: 21, Name: "Jungle",
		SurfaceBlock: 2 << 4, // grass
		FillerBlock:  3 << 4, // dirt
		BaseHeight:   70, HeightVariation: 20,
		TreeDensity:    0.12,
		BoulderDensity: 0.02,
	}
	BiomeDarkForest = &Biome{
		ID: 29, Name: "Dark Forest",
		SurfaceBlock: 2 << 4, // grass
		FillerBlock:  3 << 4, // dirt
		BaseHeight:   68, HeightVariation: 10,
		TreeDensity:    0.25,
		BoulderDensity: 0.02,
	}
	BiomeSnowyTundra = &Biome{
		ID: 12, Name: "Snowy Tundra",
		SurfaceBlock: 80 << 4, // snow block
		FillerBlock:  3 << 4,  // dirt
		BaseHeight:   66, HeightVariation: 8,
		TreeDensity:    0.004,
		BoulderDensity: 0.02,
		HasSnow:        true,
	}
)

// allBiomes is an ordered list used for selection lookups.
var allBiomes = []*Biome{
	BiomeOcean,
	BiomePlains,
	BiomeDesert,
	BiomeExtremeHills,
	BiomeForest,
	BiomeJungle,
	BiomeDarkForest,
	BiomeSnowyTundra,
}

// BiomeAt selects a biome for a world block position using temperature and
// rainfall noise values. The noise generators should use low-frequency scales
// so biomes form large regions. It uses a Whittaker-like classification to
// prevent drastic biome changes (e.g. Desert next to Tundra).
func BiomeAt(tempNoise, rainNoise *Perlin, worldX, worldZ int) *Biome {
	// Low-frequency coordinates for large biome regions
	const scale = 0.003
	bx := float64(worldX) * scale
	bz := float64(worldZ) * scale

	temp := tempNoise.OctaveNoise2D(bx, bz, 2, 2.0, 0.3) // −1..1
	rain := rainNoise.OctaveNoise2D(bx+500, bz+500, 2, 2.0, 0.3)

	// Map to 0..1 and clamp just in case
	temp = (temp + 1) / 2
	if temp < 0 {
		temp = 0
	} else if temp > 1.0 {
		temp = 1.0
	}

	rain = (rain + 1) / 2
	if rain < 0 {
		rain = 0
	} else if rain > 1.0 {
		rain = 1.0
	}

	// Whittaker Diagram Classification:
	// Tundra: Cold, any rain
	// Taiga/Extreme Hills/Forest: Temperate
	// Desert/Plains/Jungle: Warm

	switch {
	case temp < 0.35: // Cold Region
		return BiomeSnowyTundra

	case temp < 0.65: // Temperate Region
		if rain > 0.7 {
			return BiomeDarkForest // high moisture
		}
		if rain > 0.4 {
			return BiomeForest // med moisture
		}
		if rain > 0.25 {
			return BiomePlains // low moisture
		}
		return BiomeExtremeHills // very low moisture

	default: // Warm/Hot Region (temp >= 0.65)
		if rain > 0.75 {
			return BiomeJungle // high moisture, hot
		}
		// Transition between Jungle and Desert
		if rain > 0.4 {
			return BiomePlains // somewhat dry, hot
		}
		// Dry, hot
		return BiomeDesert
	}
}
