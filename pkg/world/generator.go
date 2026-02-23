package world

import (
	"bytes"
	"encoding/binary"
)

// Generator produces terrain data from a seed using Perlin noise.
type Generator struct {
	Seed         int64
	terrain      *Perlin      // broad height map noise
	roughness    *Perlin      // fine detail / roughness noise
	tempNoise    *Perlin      // biome temperature
	rainNoise    *Perlin      // biome rainfall
	caveNoise    *Perlin      // 3D cave carving
	cave2        *Perlin      // secondary 3D cave noise for spaghetti caves
	treeNoise    *Perlin      // tree placement
	boulderNoise *Perlin      // boulder placement
	villageGen   *VillageGrid // village placement grid
}

// NewGenerator creates a terrain generator from a seed.
func NewGenerator(seed int64) *Generator {
	return &Generator{
		Seed:         seed,
		terrain:      NewPerlin(seed),
		roughness:    NewPerlin(seed + 100),
		tempNoise:    NewPerlin(seed + 1),
		rainNoise:    NewPerlin(seed + 2),
		caveNoise:    NewPerlin(seed + 3),
		cave2:        NewPerlin(seed + 5),
		treeNoise:    NewPerlin(seed + 4),
		boulderNoise: NewPerlin(seed + 200),
		villageGen:   NewVillageGrid(seed),
	}
}

// FlatSurfaceY is the surface level for the flat world.
const FlatSurfaceY = 3

// SurfaceHeight returns the solid surface Y for the given world-space x, z.
func (g *Generator) SurfaceHeight(x, z int) int {
	return FlatSurfaceY
}

// isCave returns true if the block at (x,y,z) should be carved into a cave.
func (g *Generator) isCave(x, y, z int) bool {
	return false // flat world: no caves
}

// shouldPlaceTree returns true if a tree should be placed at (x, z) given the biome's density.
func (g *Generator) shouldPlaceTree(x, z int, biome *Biome) bool {
	if biome.TreeDensity <= 0 || g.villageGen.IsInVillage(x, z) {
		return false
	}
	const scale = 0.7
	v := g.treeNoise.Noise2D(float64(x)*scale, float64(z)*scale)
	v = (v + 1) / 2 // [0,1]
	return v < biome.TreeDensity
}

// WaterLevel is the sea level.
const WaterLevel = 62

// BlockAt returns the block state at a world-space (x, y, z).
// Flat world: bedrock(0), dirt(1-2), grass(3), air above.
func (g *Generator) BlockAt(x, y, z int) uint16 {
	if y < 0 || y > 255 {
		return 0
	}
	switch {
	case y == 0:
		return 7 << 4 // bedrock
	case y < FlatSurfaceY:
		return 3 << 4 // dirt
	case y == FlatSurfaceY:
		return 2 << 4 // grass
	default:
		return 0 // air
	}
}

// generateTrees places various tree types based on biome.
func (g *Generator) generateTrees(chunkX, chunkZ int, sections *[SectionsPerChunk][ChunkSectionSize]uint16) {
	for lx := 2; lx < 14; lx++ {
		for lz := 2; lz < 14; lz++ {
			wx := chunkX*16 + lx
			wz := chunkZ*16 + lz

			biome := BiomeAt(g.tempNoise, g.rainNoise, wx, wz)
			if !g.shouldPlaceTree(wx, wz, biome) {
				continue
			}

			surfaceY := g.SurfaceHeight(wx, wz)
			if surfaceY > 240 || g.isCave(wx, surfaceY, wz) {
				continue
			}

			// Surface must be grass or snow
			surfBlock := sections[surfaceY/16][(surfaceY%16*16+lz)*16+lx] >> 4
			if surfBlock != 2 && surfBlock != 80 && surfBlock != 3 { // grass, snow, or dirt
				continue
			}

			// Determine tree type
			treeType := 0 // 0: Oak, 1: Spruce, 2: Birch, 3: Jungle, 4: Dark Oak
			if biome == BiomeJungle {
				treeType = 3
			} else if biome == BiomeDarkForest {
				treeType = 4
			} else if biome == BiomeForest {
				// 30% chance of Birch in forests
				if (wx*31+wz*17)%10 < 3 {
					treeType = 2
				}
			} else if biome == BiomeExtremeHills || biome == BiomeSnowyTundra {
				treeType = 1
			}

			switch treeType {
			case 1: // Spruce
				g.buildSpruceTree(lx, surfaceY+1, lz, sections)
			case 2: // Birch
				g.buildGenericTree(lx, surfaceY+1, lz, 2, sections)
			case 3: // Jungle
				g.buildJungleTree(lx, surfaceY+1, lz, sections)
			case 4: // Dark Oak
				g.buildDarkOakTree(lx, surfaceY+1, lz, sections)
			default: // Oak
				g.buildGenericTree(lx, surfaceY+1, lz, 0, sections)
			}
		}
	}
}

// buildGenericTree builds a standard 5x5 rounded canopy tree (Oak or Birch).
func (g *Generator) buildGenericTree(lx, y, lz int, meta uint16, sections *[SectionsPerChunk][ChunkSectionSize]uint16) {
	trunkTop := y + 3
	// Trunk
	for ty := y; ty <= trunkTop+1; ty++ {
		sec, sy := ty/16, ty%16
		current := sections[sec][(sy*16+lz)*16+lx] >> 4
		if current == 0 || current == 31 || current == 18 || current == 161 {
			sections[sec][(sy*16+lz)*16+lx] = 17<<4 | meta
		}
	}
	// Leaves
	leafBase := uint16(18<<4 | meta)
	for dy := -1; dy <= 0; dy++ {
		ly := trunkTop + dy
		sec, sy := ly/16, ly%16
		for dx := -2; dx <= 2; dx++ {
			for dz := -2; dz <= 2; dz++ {
				if (dx == -2 || dx == 2) && (dz == -2 || dz == 2) {
					continue
				}
				nlx, nlz := lx+dx, lz+dz
				if nlx >= 0 && nlx < 16 && nlz >= 0 && nlz < 16 {
					if sections[sec][(sy*16+nlz)*16+nlx] == 0 {
						sections[sec][(sy*16+nlz)*16+nlx] = leafBase
					}
				}
			}
		}
	}
	for dy := 1; dy <= 2; dy++ {
		ly := trunkTop + dy
		sec, sy := ly/16, ly%16
		for dx := -1; dx <= 1; dx++ {
			for dz := -1; dz <= 1; dz++ {
				if dy == 2 && dx != 0 && dz != 0 {
					continue
				}
				nlx, nlz := lx+dx, lz+dz
				if nlx >= 0 && nlx < 16 && nlz >= 0 && nlz < 16 {
					if sections[sec][(sy*16+nlz)*16+nlx] == 0 {
						sections[sec][(sy*16+nlz)*16+nlx] = leafBase
					}
				}
			}
		}
	}
}

// buildSpruceTree builds a conical spruce tree.
func (g *Generator) buildSpruceTree(lx, y, lz int, sections *[SectionsPerChunk][ChunkSectionSize]uint16) {
	height := 5 + (lx*13+lz*7)%3
	trunkTop := y + height - 1
	// Trunk
	for ty := y; ty <= trunkTop; ty++ {
		sec, sy := ty/16, ty%16
		current := sections[sec][(sy*16+lz)*16+lx] >> 4
		if current == 0 || current == 31 || current == 18 || current == 161 {
			sections[sec][(sy*16+lz)*16+lx] = 17<<4 | 1 // Spruce log
		}
	}
	// Leaves
	leafBase := uint16(18<<4 | 1) // Spruce leaves
	for dy := 2; dy <= height; dy++ {
		ly := y + dy
		sec, sy := ly/16, ly%16
		radius := 2
		if dy > height-2 {
			radius = 0
		} else if dy > height-4 {
			radius = 1
		}
		for dx := -radius; dx <= radius; dx++ {
			for dz := -radius; dz <= radius; dz++ {
				if radius > 1 && (dx == -radius || dx == radius) && (dz == -radius || dz == radius) {
					continue
				}
				nlx, nlz := lx+dx, lz+dz
				if nlx >= 0 && nlx < 16 && nlz >= 0 && nlz < 16 {
					if sections[sec][(sy*16+nlz)*16+nlx] == 0 {
						sections[sec][(sy*16+nlz)*16+nlx] = leafBase
					}
				}
			}
		}
	}
}

// buildJungleTree builds a very tall jungle tree.
func (g *Generator) buildJungleTree(lx, y, lz int, sections *[SectionsPerChunk][ChunkSectionSize]uint16) {
	height := 8 + (lx*7+lz*13)%6
	trunkTop := y + height - 1
	// Trunk
	for ty := y; ty <= trunkTop; ty++ {
		sec, sy := ty/16, ty%16
		current := sections[sec][(sy*16+lz)*16+lx] >> 4
		if current == 0 || current == 31 || current == 18 || current == 161 {
			sections[sec][(sy*16+lz)*16+lx] = 17<<4 | 3 // Jungle log
		}
	}
	// Leaves (multiple layers)
	leafBase := uint16(18<<4 | 3) // Jungle leaves
	for dy := height - 3; dy <= height; dy++ {
		radius := 2
		if dy == height {
			radius = 1
		}
		ly := y + dy
		sec, sy := ly/16, ly%16
		for dx := -radius; dx <= radius; dx++ {
			for dz := -radius; dz <= radius; dz++ {
				if (dx*dx + dz*dz) > radius*radius+1 {
					continue
				}
				nlx, nlz := lx+dx, lz+dz
				if nlx >= 0 && nlx < 16 && nlz >= 0 && nlz < 16 {
					if sections[sec][(sy*16+nlz)*16+nlx] == 0 {
						sections[sec][(sy*16+nlz)*16+nlx] = leafBase
					}
				}
			}
		}
	}
}

// buildDarkOakTree builds a 2x2 trunk tree with a broad canopy.
func (g *Generator) buildDarkOakTree(lx, y, lz int, sections *[SectionsPerChunk][ChunkSectionSize]uint16) {
	height := 6 + (lx*3+lz*5)%3
	trunkTop := y + height - 1
	// 2x2 Trunk (if space permits)
	for dx := 0; dx <= 1; dx++ {
		for dz := 0; dz <= 1; dz++ {
			nlx, nlz := lx+dx, lz+dz
			if nlx < 16 && nlz < 16 {
				for ty := y; ty <= trunkTop; ty++ {
					sec, sy := ty/16, ty%16
					current := sections[sec][(sy*16+nlz)*16+nlx] >> 4
					if current == 0 || current == 31 || current == 18 || current == 161 {
						sections[sec][(sy*16+nlz)*16+nlx] = 162<<4 | 1 // Dark Oak log
					}
				}
			}
		}
	}
	// Broad canopy
	leafBase := uint16(161<<4 | 1) // Dark Oak leaves
	for dy := height - 3; dy <= height; dy++ {
		ly := y + dy
		sec, sy := ly/16, ly%16
		radius := 3
		if dy == height {
			radius = 2
		}
		for dx := -radius + 1; dx <= radius; dx++ {
			for dz := -radius + 1; dz <= radius; dz++ {
				if (dx*dx + dz*dz) > radius*radius+2 {
					continue
				}
				nlx, nlz := lx+dx, lz+dz
				if nlx >= 0 && nlx < 16 && nlz >= 0 && nlz < 16 {
					if sections[sec][(sy*16+nlz)*16+nlx] == 0 {
						sections[sec][(sy*16+nlz)*16+nlx] = leafBase
					}
				}
			}
		}
	}
}

// generateBoulders places varied rock clusters.
func (g *Generator) generateBoulders(chunkX, chunkZ int, sections *[SectionsPerChunk][ChunkSectionSize]uint16) {
	for lx := 1; lx < 15; lx++ {
		for lz := 1; lz < 15; lz++ {
			wx, wz := chunkX*16+lx, chunkZ*16+lz
			biome := BiomeAt(g.tempNoise, g.rainNoise, wx, wz)
			if biome.BoulderDensity <= 0 {
				continue
			}
			v := g.boulderNoise.Noise2D(float64(wx)*0.1, float64(wz)*0.1)
			if v < 1.0-biome.BoulderDensity*5 || g.villageGen.IsInVillage(wx, wz) {
				continue
			}

			y := g.SurfaceHeight(wx, wz)
			sec, sy := y/16, y%16
			if sections[sec][(sy*16+lz)*16+lx]>>4 != 2 && sections[sec][(sy*16+lz)*16+lx]>>4 != 3 {
				continue
			}

			// Create larger organic boulder
			hr := wx*31 + wz*17
			if hr < 0 {
				hr = -hr
			}
			baseRadius := 3.0 + float64(hr%3) // Radius 3 to 5
			for dx := -int(baseRadius) - 1; dx <= int(baseRadius)+1; dx++ {
				for dy := -1; dy <= int(baseRadius); dy++ {
					for dz := -int(baseRadius) - 1; dz <= int(baseRadius)+1; dz++ {
						// Squish the sphere down slightly for a flatter look
						distSq := float64(dx*dx)/(baseRadius*baseRadius) +
							float64(dy*dy)/((baseRadius-0.5)*(baseRadius-0.5)) +
							float64(dz*dz)/(baseRadius*baseRadius)

						// Add some noise to the edges for a natural shape
						noiseOff := float64((wx+dx*7+wz+dz*11+dy*13)%100) / 100.0 * 0.4
						if distSq+noiseOff > 1.0 {
							continue
						}

						nlx, nlz := lx+dx, lz+dz
						if nlx < 0 || nlx >= 16 || nlz < 0 || nlz >= 16 {
							continue
						}

						targetY := y + dy
						if targetY < 0 || targetY > 255 {
							continue
						}

						// Material variety matching the requested palette
						h := (wx + dx*31 + wz*dz*17 + dy*23)
						block := uint16(4 << 4) // default cobblestone
						r := h % 100
						if r < 30 {
							block = 1 << 4 // 30% stone
						} else if r < 60 {
							block = 1<<4 | 5 // 30% andesite (stone meta 5)
						} else if r < 80 {
							block = 48 << 4 // 20% mossy cobblestone
						} // Remaining 20% is cobblestone

						// Place the block if the location is air, grass, foliage, or dirt
						targetSec, targetSy := targetY/16, targetY%16
						targetID := sections[targetSec][(targetSy*16+nlz)*16+nlx] >> 4
						if targetID == 0 || targetID == 2 || targetID == 31 || targetID == 3 {
							sections[targetSec][(targetSy*16+nlz)*16+nlx] = block
						}
					}
				}
			}
		}
	}
}

// GenerateInternal produces the raw chunk data (sections and biomes).
func (g *Generator) GenerateInternal(chunkX, chunkZ int) ([SectionsPerChunk][ChunkSectionSize]uint16, [256]byte) {
	var sections [SectionsPerChunk][ChunkSectionSize]uint16
	var biomes [256]byte

	// Fill block data per section — flat world
	for lx := 0; lx < 16; lx++ {
		for lz := 0; lz < 16; lz++ {
			biomes[lz*16+lx] = BiomeAt(g.tempNoise, g.rainNoise, chunkX*16+lx, chunkZ*16+lz).ID

			for y := 0; y <= FlatSurfaceY; y++ {
				sec := y / 16
				sy := y % 16
				idx := (sy*16+lz)*16 + lx

				switch {
				case y == 0:
					sections[sec][idx] = 7 << 4 // bedrock
				case y < FlatSurfaceY:
					sections[sec][idx] = 3 << 4 // dirt
				case y == FlatSurfaceY:
					sections[sec][idx] = 2 << 4 // grass
				}
			}
		}
	}

	// Place village structures
	g.villageGen.generateVillage(chunkX, chunkZ, FlatSurfaceY, &sections)

	// Place boulders
	g.generateBoulders(chunkX, chunkZ, &sections)

	// Place trees
	g.generateTrees(chunkX, chunkZ, &sections)

	return sections, biomes
}

// GenerateChunkData produces the complete 1.8 chunk column data for (chunkX, chunkZ).
func (g *Generator) GenerateChunkData(chunkX, chunkZ int) ([]byte, uint16) {
	sections, biomes := g.GenerateInternal(chunkX, chunkZ)
	return SerializeSections(&sections, biomes)
}

// SerializeSections converts populated section arrays into 1.8 chunk wire format.
func SerializeSections(sections *[SectionsPerChunk][ChunkSectionSize]uint16, biomes [256]byte) ([]byte, uint16) {
	var primaryBitMask uint16
	var buf bytes.Buffer

	// First pass: determine which sections are non-empty
	for s := 0; s < SectionsPerChunk; s++ {
		empty := true
		for _, b := range sections[s] {
			if b != 0 {
				empty = false
				break
			}
		}
		if !empty {
			primaryBitMask |= 1 << uint(s)
		}
	}

	// Write all block data for each active section first
	for s := 0; s < SectionsPerChunk; s++ {
		if primaryBitMask&(1<<uint(s)) == 0 {
			continue
		}
		for _, b := range sections[s] {
			binary.Write(&buf, binary.LittleEndian, b)
		}
	}

	// Then write block light for each active section
	for s := 0; s < SectionsPerChunk; s++ {
		if primaryBitMask&(1<<uint(s)) == 0 {
			continue
		}
		light := make([]byte, 2048)
		for i := range light {
			light[i] = 0xFF
		}
		buf.Write(light)
	}

	// Then write sky light for each active section
	for s := 0; s < SectionsPerChunk; s++ {
		if primaryBitMask&(1<<uint(s)) == 0 {
			continue
		}
		sky := make([]byte, 2048)
		for i := range sky {
			sky[i] = 0xFF
		}
		buf.Write(sky)
	}

	// Biome data (256 bytes)
	buf.Write(biomes[:])

	return buf.Bytes(), primaryBitMask
}
