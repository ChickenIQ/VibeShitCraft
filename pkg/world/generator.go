package world

import (
	"bytes"
	"encoding/binary"
)

// Generator produces terrain data from a seed using Perlin noise.
type Generator struct {
	Seed      int64
	terrain   *Perlin // broad height map noise
	roughness *Perlin // fine detail / roughness noise
	tempNoise *Perlin // biome temperature
	rainNoise *Perlin // biome rainfall
	caveNoise *Perlin // 3D cave carving
	cave2     *Perlin // secondary 3D cave noise for spaghetti caves
	treeNoise *Perlin // tree placement
}

// NewGenerator creates a terrain generator from a seed.
func NewGenerator(seed int64) *Generator {
	return &Generator{
		Seed:      seed,
		terrain:   NewPerlin(seed),
		roughness: NewPerlin(seed + 100),
		tempNoise: NewPerlin(seed + 1),
		rainNoise: NewPerlin(seed + 2),
		caveNoise: NewPerlin(seed + 3),
		cave2:     NewPerlin(seed + 5),
		treeNoise: NewPerlin(seed + 4),
	}
}

// SurfaceHeight returns the solid surface Y for the given world-space x, z.
func (g *Generator) SurfaceHeight(x, z int) int {
	biome := BiomeAt(g.tempNoise, g.rainNoise, x, z)

	// Broad terrain shape — large rolling hills/valleys
	const broadScale = 0.005
	n := g.terrain.OctaveNoise2D(float64(x)*broadScale, float64(z)*broadScale, 6, 2.0, 0.5)
	// n is in [-1, 1] — map to [0, 1]
	n = (n + 1) / 2

	// Fine detail roughness — smaller bumps and ridges
	const detailScale = 0.03
	detail := g.roughness.OctaveNoise2D(float64(x)*detailScale, float64(z)*detailScale, 4, 2.0, 0.5)
	// detail is in [-1, 1] — scale to ±4 blocks of micro variation
	detailHeight := detail * 4.0

	// Medium scale hills — intermediate features
	const medScale = 0.012
	med := g.terrain.OctaveNoise2D(float64(x)*medScale+1000, float64(z)*medScale+1000, 4, 2.0, 0.5)
	medHeight := med * biome.HeightVariation * 0.3

	height := biome.BaseHeight + int(n*biome.HeightVariation) + int(medHeight) + int(detailHeight)
	if height < 1 {
		height = 1
	}
	if height > 250 {
		height = 250
	}
	return height
}

// isCave returns true if the block at (x,y,z) should be carved into a cave.
func (g *Generator) isCave(x, y, z int) bool {
	if y <= 0 || y >= 128 {
		return false // no caves at bedrock or high up
	}
	// Primary cave noise — creates large caverns
	const scale = 0.04
	v := g.caveNoise.Noise3D(float64(x)*scale, float64(y)*scale*0.5, float64(z)*scale)

	// Secondary cave noise — creates thinner connecting tunnels
	const scale2 = 0.08
	v2 := g.cave2.Noise3D(float64(x)*scale2, float64(y)*scale2*0.7, float64(z)*scale2)

	// Combine: either large cavern or spaghetti tunnel
	return v > 0.45 || (v2 > 0.55 && v > 0.2)
}

// shouldPlaceTree returns true if a tree should be placed at (x, z) given the biome's density.
func (g *Generator) shouldPlaceTree(x, z int, biome *Biome) bool {
	if biome.TreeDensity <= 0 {
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
// This is the authoritative generator lookup used by World.GetBlock as fallback.
func (g *Generator) BlockAt(x, y, z int) uint16 {
	if y < 0 || y > 255 {
		return 0
	}
	if y == 0 {
		return 7 << 4 // bedrock
	}

	biome := BiomeAt(g.tempNoise, g.rainNoise, x, z)
	surfaceY := g.SurfaceHeight(x, z)

	// Cave carving
	if g.isCave(x, y, z) && y > 1 && y < surfaceY-1 {
		if y <= WaterLevel {
			return 9 << 4 // water in low caves
		}
		return 0 // air
	}

	if y < surfaceY-4 {
		return 1 << 4 // stone
	}
	if y < surfaceY {
		return biome.FillerBlock
	}
	if y == surfaceY {
		// Underwater surfaces get sand/gravel instead of grass
		if surfaceY < WaterLevel {
			return 12 << 4 // sand
		}
		return biome.SurfaceBlock
	}

	// Above surface
	if y <= WaterLevel {
		return 9 << 4 // water (still)
	}

	return 0 // air
}

// generateTrees places simple oak trees into the sections array for a chunk.
// treeMap marks which columns have a tree so we can overlap leaves correctly.
func (g *Generator) generateTrees(chunkX, chunkZ int, sections *[SectionsPerChunk][ChunkSectionSize]uint16) {
	for lx := 2; lx < 14; lx++ { // keep 2-block margin for leaves
		for lz := 2; lz < 14; lz++ {
			wx := chunkX*16 + lx
			wz := chunkZ*16 + lz

			biome := BiomeAt(g.tempNoise, g.rainNoise, wx, wz)
			if !g.shouldPlaceTree(wx, wz, biome) {
				continue
			}

			surfaceY := g.SurfaceHeight(wx, wz)

			// Only on exposed surface above water
			if surfaceY <= WaterLevel || surfaceY > 240 {
				continue
			}

			// Don't place in caves
			if g.isCave(wx, surfaceY, wz) {
				continue
			}

			// Surface must be grass or snow to place tree
			surfBlock := biome.SurfaceBlock >> 4
			if surfBlock != 2 && surfBlock != 80 {
				continue
			}

			// Place trunk (4 blocks of oak log, block ID 17)
			trunkBase := surfaceY + 1
			trunkTop := trunkBase + 3
			for ty := trunkBase; ty <= trunkTop+1; ty++ {
				if ty > 255 {
					break
				}
				sec := ty / 16
				sy := ty % 16
				idx := (sy*16+lz)*16 + lx
				sections[sec][idx] = 17 << 4 // oak log
			}

			// Place leaves (oak leaves = block ID 18)
			leafBlock := uint16(18 << 4)
			// Layer at trunkTop-1 and trunkTop: 5x5 cross with corners
			for dy := -1; dy <= 0; dy++ {
				ly := trunkTop + dy
				if ly < 0 || ly > 255 {
					continue
				}
				sec := ly / 16
				sy := ly % 16
				for dx := -2; dx <= 2; dx++ {
					for dz := -2; dz <= 2; dz++ {
						nlx := lx + dx
						nlz := lz + dz
						if nlx < 0 || nlx > 15 || nlz < 0 || nlz > 15 {
							continue
						}
						// Skip corners of 5x5
						if (dx == -2 || dx == 2) && (dz == -2 || dz == 2) {
							continue
						}
						idx := (sy*16+nlz)*16 + nlx
						if sections[sec][idx] == 0 {
							sections[sec][idx] = leafBlock
						}
					}
				}
			}
			// Layer at trunkTop+1: 3x3 cross
			for dy := 1; dy <= 2; dy++ {
				ly := trunkTop + dy
				if ly < 0 || ly > 255 {
					continue
				}
				sec := ly / 16
				sy := ly % 16
				for dx := -1; dx <= 1; dx++ {
					for dz := -1; dz <= 1; dz++ {
						if dx != 0 && dz != 0 && dy == 2 {
							continue // top layer is just a +
						}
						nlx := lx + dx
						nlz := lz + dz
						if nlx < 0 || nlx > 15 || nlz < 0 || nlz > 15 {
							continue
						}
						idx := (sy*16+nlz)*16 + nlx
						if sections[sec][idx] == 0 {
							sections[sec][idx] = leafBlock
						}
					}
				}
			}
		}
	}
}

// GenerateChunkData produces the complete 1.8 chunk column data for (chunkX, chunkZ).
func (g *Generator) GenerateChunkData(chunkX, chunkZ int) ([]byte, uint16) {
	var sections [SectionsPerChunk][ChunkSectionSize]uint16
	var biomes [256]byte

	// Fill block data per section
	for lx := 0; lx < 16; lx++ {
		for lz := 0; lz < 16; lz++ {
			wx := chunkX*16 + lx
			wz := chunkZ*16 + lz

			biome := BiomeAt(g.tempNoise, g.rainNoise, wx, wz)
			biomes[lz*16+lx] = biome.ID

			surfaceY := g.SurfaceHeight(wx, wz)

			for y := 0; y < ChunkHeight; y++ {
				sec := y / 16
				sy := y % 16
				idx := (sy*16+lz)*16 + lx

				if y == 0 {
					sections[sec][idx] = 7 << 4 // bedrock
					continue
				}

				// Cave carving
				if g.isCave(wx, y, wz) && y > 1 && y < surfaceY-1 {
					if y <= WaterLevel {
						sections[sec][idx] = 9 << 4 // water
					} else {
						sections[sec][idx] = 0 // air
					}
					continue
				}

				if y < surfaceY-4 {
					sections[sec][idx] = 1 << 4 // stone
				} else if y < surfaceY {
					sections[sec][idx] = biome.FillerBlock
				} else if y == surfaceY {
					if surfaceY < WaterLevel {
						sections[sec][idx] = 12 << 4 // sand underwater
					} else {
						sections[sec][idx] = biome.SurfaceBlock
					}
				} else if y <= WaterLevel {
					sections[sec][idx] = 9 << 4 // water
				} else {
					sections[sec][idx] = 0 // air
				}
			}
		}
	}

	// Place trees
	g.generateTrees(chunkX, chunkZ, &sections)

	// Serialize
	return serializeSections(&sections, biomes)
}

// serializeSections converts populated section arrays into 1.8 chunk wire format.
func serializeSections(sections *[SectionsPerChunk][ChunkSectionSize]uint16, biomes [256]byte) ([]byte, uint16) {
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

	// Write block data for each active section
	for s := 0; s < SectionsPerChunk; s++ {
		if primaryBitMask&(1<<uint(s)) == 0 {
			continue
		}
		for _, b := range sections[s] {
			binary.Write(&buf, binary.LittleEndian, b)
		}

		// Block light (2048 bytes) — full bright
		light := make([]byte, 2048)
		for i := range light {
			light[i] = 0xFF
		}
		buf.Write(light)

		// Sky light (2048 bytes) — full bright
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
