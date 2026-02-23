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

	// Fill block data per section — flat world
	for lx := 0; lx < 16; lx++ {
		for lz := 0; lz < 16; lz++ {
			biomes[lz*16+lx] = 1 // Plains biome everywhere

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
