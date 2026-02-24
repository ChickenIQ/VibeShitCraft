package world

// VillageGrid handles deterministic village placement on a sparse grid.
// Every VILLAGE_CELL_SIZE blocks in X and Z forms a grid cell; each cell
// independently rolls whether it contains a village center.
const villageCellSize = 96

// VillageGrid carries the world seed used for per-cell rolls.
type VillageGrid struct {
	seed int64
}

// NewVillageGrid creates a VillageGrid for the given world seed.
func NewVillageGrid(seed int64) *VillageGrid {
	return &VillageGrid{seed: seed}
}

// cellHash returns a deterministic value in [0, mod) for cell (cx, cz).
func (v *VillageGrid) cellHash(cx, cz, mod int64) int64 {
	const k1 int64 = -7046029254386353131 // splitmix64 step 1
	const k2 int64 = -4265267296055464877 // splitmix64 step 2
	h := v.seed ^ (cx * k1) ^ (cz * 7823434773480878946)
	h ^= h >> 33
	h *= k1
	h ^= h >> 27
	h *= k2
	h ^= h >> 31
	if h < 0 {
		h = -h
	}
	return h % mod
}

// villageCenter returns the world-space (x, z) center of a village in grid cell
// (cellX, cellZ), and ok=true if that cell contains a village (25% chance).
func (v *VillageGrid) villageCenter(cellX, cellZ int) (wx, wz int, ok bool) {
	cx := int64(cellX)
	cz := int64(cellZ)
	// ~25% of cells get a village
	if v.cellHash(cx, cz, 4) != 0 {
		return 0, 0, false
	}
	// Offset within the cell so villages aren't always at the corner
	ox := int(v.cellHash(cx^0xDEAD, cz^0xBEEF, int64(villageCellSize-20))) + 10
	oz := int(v.cellHash(cx^0xCAFE, cz^0xF00D, int64(villageCellSize-20))) + 10
	return cellX*villageCellSize + ox, cellZ*villageCellSize + oz, true
}

// divFloor returns a / b, rounding towards negative infinity.
func divFloor(a, b int) int {
	if a < 0 && a%b != 0 {
		return a/b - 1
	}
	return a / b
}

// generateVillage writes village structures into sections for chunk (chunkX, chunkZ).
// surfY is the flat surface Y (the grass layer). All coordinates stay inside
// the section array exactly like generateTrees does.
func (v *VillageGrid) generateVillage(chunkX, chunkZ, surfY int, sections *[SectionsPerChunk][ChunkSectionSize]uint16) {
	// Chunk world-space bounds
	minWX := chunkX * 16
	minWZ := chunkZ * 16
	maxWX := minWX + 15
	maxWZ := minWZ + 15

	// Village influence radius (structures can reach this many blocks from center)
	// Increased to 48 to capture large halls and branching paths.
	const radius = 48

	// Determine which grid cells could have villages whose structures overlap this chunk.
	// We use floor division to ensure correct mapping in all coordinate quadrants.
	cellMinX := divFloor(minWX-radius, villageCellSize)
	cellMaxX := divFloor(maxWX+radius, villageCellSize)
	cellMinZ := divFloor(minWZ-radius, villageCellSize)
	cellMaxZ := divFloor(maxWZ+radius, villageCellSize)

	for cx := cellMinX; cx <= cellMaxX; cx++ {
		for cz := cellMinZ; cz <= cellMaxZ; cz++ {
			vx, vz, ok := v.villageCenter(cx, cz)
			if !ok {
				continue
			}
			v.buildVillage(vx, vz, surfY, chunkX, chunkZ, sections)
		}
	}
}

// IsInVillage returns true if (wx, wz) is within the influence area of a village.
func (v *VillageGrid) IsInVillage(wx, wz int) bool {
	const influenceRadius = 40
	cellMinX := divFloor(wx-influenceRadius, villageCellSize)
	cellMaxX := divFloor(wx+influenceRadius, villageCellSize)
	cellMinZ := divFloor(wz-influenceRadius, villageCellSize)
	cellMaxZ := divFloor(wz+influenceRadius, villageCellSize)

	for cx := cellMinX; cx <= cellMaxX; cx++ {
		for cz := cellMinZ; cz <= cellMaxZ; cz++ {
			vx, vz, ok := v.villageCenter(cx, cz)
			if !ok {
				continue
			}
			dx := wx - vx
			dz := wz - vz
			if dx >= -influenceRadius && dx <= influenceRadius && dz >= -influenceRadius && dz <= influenceRadius {
				return true
			}
		}
	}
	return false
}

// setBlock is a helper that writes block state into the sections array, clamping
// to valid section / local coordinates within the chunk.
func setBlock(lx, y, lz int, state uint16, sections *[SectionsPerChunk][ChunkSectionSize]uint16) {
	if y < 0 || y > 255 || lx < 0 || lx > 15 || lz < 0 || lz > 15 {
		return
	}
	sec := y / 16
	sy := y % 16
	sections[sec][(sy*16+lz)*16+lx] = state
}

// worldToLocal converts a world X or Z to the local coordinate within chunkX/chunkZ.
// Returns the local coord and whether it falls within [0,15].
func worldToLocal(world, chunkOrigin int) (int, bool) {
	l := world - chunkOrigin
	return l, l >= 0 && l <= 15
}

// buildVillage places all structures for a village centered at (vx, vz, surfY)
// into the sections array, writing only blocks that fall within this chunk.
func (v *VillageGrid) buildVillage(vx, vz, surfY, chunkX, chunkZ int, sections *[SectionsPerChunk][ChunkSectionSize]uint16) {
	originX := chunkX * 16
	originZ := chunkZ * 16

	// Block IDs (state = id << 4)
	const (
		air          = 0
		log          = 17 << 4  // oak log
		cobble       = 4 << 4   // cobblestone
		planks       = 5 << 4   // oak planks
		gravel       = 13 << 4  // gravel (path)
		glass        = 20 << 4  // glass pane (window)
		fence        = 85 << 4  // oak fence
		stoneBrick   = 98 << 4  // stone bricks (well walls)
		cobbleWall   = 139 << 4 // cobblestone wall
		waterSrc     = 9 << 4   // water (stationary)
		torch        = 50 << 4  // torch
		slab         = 44 << 4  // stone slab
		oakStairs    = 53 << 4  // oak stairs
		woodenDoor   = 64 << 4  // wooden door
		farmland     = 60 << 4  // farmland
		wheat        = 59 << 4  // wheat
		carrot       = 141 << 4 // carrot
		potato       = 142 << 4 // potato
		melonStem    = 105 << 4 // melon stem
		pumpkinStem  = 104 << 4 // pumpkin stem
		melonBlock   = 103 << 4 // melon block
		pumpkinBlock = 86 << 4  // pumpkin block
		dandelion    = 37 << 4  // dandelion flower
		poppy        = 38 << 4  // poppy flower
		goldBlock    = 41 << 4  // gold block (bell)
		oakSlab      = 126 << 4 // oak slab
		bed          = 26 << 4  // bed
	)

	placeBlock := func(wx, y, wz int, state uint16) {
		lx := wx - originX
		lz := wz - originZ
		setBlock(lx, y, lz, state, sections)
	}

	// isSafeToDecorate returns true if the block at (wx, y, wz) is air or gravel.
	isSafeToDecorate := func(wx, y, wz int) bool {
		lx := wx - originX
		lz := wz - originZ
		if y < 0 || y > 255 || lx < 0 || lx > 15 || lz < 0 || lz > 15 {
			return false
		}
		sec := y / 16
		sy := y % 16
		currentState := sections[sec][(sy*16+lz)*16+lx]
		return (currentState>>4) == 0 || (currentState>>4) == 13 // Air (0) or Gravel (13)
	}

	placeDecoration := func(wx, y, wz int, state uint16) {
		if isSafeToDecorate(wx, y, wz) {
			placeBlock(wx, y, wz, state)
		}
	}

	// isSafeForPath returns true if the block at (wx, y, wz) is air, grass, or dirt.
	isSafeForPath := func(wx, y, wz int) bool {
		lx := wx - originX
		lz := wz - originZ
		if y < 0 || y > 255 || lx < 0 || lx > 15 || lz < 0 || lz > 15 {
			return false
		}
		sec := y / 16
		sy := y % 16
		currentState := sections[sec][(sy*16+lz)*16+lx]
		id := currentState >> 4
		return id == 0 || id == 2 || id == 3 || id == 13 // Air, Grass, Dirt, Gravel
	}

	placePath := func(wx, y, wz int, state uint16) {
		if isSafeForPath(wx, y, wz) {
			placeBlock(wx, y, wz, state)
		}
	}

	// ------------------------------------------------------------------
	// 1. Gravel paths: Main cross + secondary branches
	// ------------------------------------------------------------------
	const pathLen = 32
	for d := -pathLen; d <= pathLen; d++ {
		for w := -1; w <= 1; w++ {
			// Main X axis
			placePath(vx+d, surfY, vz+w, gravel)
			// Main Z axis
			placePath(vx+w, surfY, vz+d, gravel)
		}
	}
	// Secondary branches
	for d := 8; d <= 24; d++ {
		for w := -1; w <= 1; w++ {
			// Branch at X=16, going Z
			if d >= 16 {
				placePath(vx+16+w, surfY, vz+d, gravel)
				placePath(vx-16+w, surfY, vz-d, gravel)
			}
			// Branch at Z=16, going X
			if d >= 16 {
				placePath(vx+d, surfY, vz+16+w, gravel)
				placePath(vx-d, surfY, vz-16+w, gravel)
			}
		}
	}

	// ------------------------------------------------------------------
	// 2. Well (at center) - 4x4 Design with Slabs and Pillars
	// ------------------------------------------------------------------
	// Gravel foundation/path around the well (6x6)
	for dx := -3; dx <= 2; dx++ {
		for dz := -3; dz <= 2; dz++ {
			placePath(vx+dx, surfY, vz+dz, gravel)
		}
	}

	for dx := -2; dx <= 1; dx++ {
		for dz := -2; dz <= 1; dz++ {
			isCorner := (dx == -2 || dx == 1) && (dz == -2 || dz == 1)
			isInner := (dx == -1 || dx == 0) && (dz == -1 || dz == 0)

			// Base layer (4x4 stone brick)
			placeBlock(vx+dx, surfY+1, vz+dz, stoneBrick)

			// Second layer: Pillars at corners, stone bricks at edges, water in middle
			if isCorner {
				// 3-high cobblestone pillars
				placeBlock(vx+dx, surfY+2, vz+dz, cobbleWall)
				placeBlock(vx+dx, surfY+3, vz+dz, cobbleWall)
				placeBlock(vx+dx, surfY+4, vz+dz, cobbleWall)
			} else if isInner {
				// 2x2 Water center
				placeBlock(vx+dx, surfY+1, vz+dz, stoneBrick) // floor of well
				placeBlock(vx+dx, surfY+2, vz+dz, waterSrc)
			} else {
				// Edge blocks (stone bricks)
				placeBlock(vx+dx, surfY+2, vz+dz, stoneBrick)
			}

			// Roof (4x4 stone slabs)
			placeBlock(vx+dx, surfY+5, vz+dz, slab)
		}
	}

	// ------------------------------------------------------------------
	// 3. Structures: Large Hall, Houses, and Farms
	// ------------------------------------------------------------------
	// Large Hall (North) - Shifted to vx-3 so door (hx+3) is at vx center path
	v.buildLargeHall(vx-3, vz+22, surfY, placeBlock, log, planks, cobble, glass, torch, oakStairs, woodenDoor, air, oakSlab, bed)

	// Small Houses
	v.buildHouse(vx+12, vz+6, surfY, 0, placeBlock, log, planks, cobble, glass, fence, torch, oakStairs, woodenDoor, air, oakSlab, bed)
	v.buildHouse(vx-12, vz-6, surfY, 1, placeBlock, log, planks, cobble, glass, fence, torch, oakStairs, woodenDoor, air, oakSlab, bed)
	v.buildHouse(vx+22, vz-12, surfY, 2, placeBlock, log, planks, cobble, glass, fence, torch, oakStairs, woodenDoor, air, oakSlab, bed)

	// Farms
	// Potential farm locations (avoiding main buildings)
	farmLocations := []struct{ dx, dz int }{
		{-22, 12}, // West branch area
		{6, -22},  // South-ish branch area
		{22, 12},  // East branch area
		{-6, -30}, // Far South branch area
	}

	// Deterministically decide how many farms to build (1 to 4)
	numFarms := int(v.cellHash(int64(vx), int64(vz), 4)) + 1

	// Shuffle locations deterministically for variety in placement
	for i := len(farmLocations) - 1; i > 0; i-- {
		j := int(v.cellHash(int64(vx+i), int64(vz-i), int64(i+1)))
		farmLocations[i], farmLocations[j] = farmLocations[j], farmLocations[i]
	}

	for i := 0; i < numFarms; i++ {
		fx, fz := vx+farmLocations[i].dx, vz+farmLocations[i].dz
		farmHash := v.cellHash(int64(fx), int64(fz), 10)

		if farmHash < 7 { // 70% chance for standard crop farm
			crops := []uint16{wheat, carrot, potato}
			v.buildFarm(fx, fz, surfY, placeBlock, log, waterSrc, farmland, crops[farmHash%3])
		} else { // 30% chance for fruit farm
			fruits := []uint16{melonBlock, pumpkinBlock}
			stems := []uint16{melonStem, pumpkinStem}
			idx := int(farmHash % 2)
			v.buildFruitFarm(fx, fz, surfY, placeBlock, log, waterSrc, farmland, stems[idx], fruits[idx])
		}
	}

	// Church (at West)
	v.buildChurch(vx-18, vz-18, surfY, placeBlock, log, planks, cobble, glass, torch, oakStairs, woodenDoor, slab, air, goldBlock)

	// ------------------------------------------------------------------
	// 4. Decorations: Flowers & Lamp Posts
	// ------------------------------------------------------------------
	// Flowers randomly around structures
	for i := -30; i <= 30; i += 7 {
		for j := -30; j <= 30; j += 7 {
			h := int64(i*131 + j*17)
			if h%5 == 0 {
				flower := dandelion
				if h%10 == 0 {
					flower = poppy
				}
				placeDecoration(vx+i, surfY+1, vz+j, uint16(flower))
			}
		}
	}

	// Lamp posts along the main paths
	for _, dist := range []int{10, 20, 30} {
		for _, axis := range [][2]int{{dist, 2}, {-dist, -2}, {2, dist}, {-2, -dist}} {
			px, pz := vx+axis[0], vz+axis[1]
			// 3-high fence post
			placeDecoration(px, surfY+1, pz, fence)
			placeDecoration(px, surfY+2, pz, fence)
			placeDecoration(px, surfY+3, pz, fence)
			// Torch on top (meta 5 = upright on top of block)
			placeDecoration(px, surfY+4, pz, torch|5)
		}
	}
}

// buildFarm creates a 7x7 farm with a water trench in the middle.
func (v *VillageGrid) buildFarm(fx, fz, surfY int, place func(wx, y, wz int, state uint16), log, water, farmland, crop uint16) {
	for dx := -3; dx <= 3; dx++ {
		for dz := -3; dz <= 3; dz++ {
			isBorder := dx == -3 || dx == 3 || dz == -3 || dz == 3
			if isBorder {
				place(fx+dx, surfY, fz+dz, log)
			} else if dx == 0 {
				place(fx+dx, surfY, fz+dz, water)
			} else {
				place(fx+dx, surfY, fz+dz, farmland)
				place(fx+dx, surfY+1, fz+dz, crop|7) // fully grown (meta 7)
			}
		}
	}
}

// buildFruitFarm creates a 7x7 farm designed for melons or pumpkins.
func (v *VillageGrid) buildFruitFarm(fx, fz, surfY int, place func(wx, y, wz int, state uint16), log, water, farmland, stem, fruit uint16) {
	for dx := -3; dx <= 3; dx++ {
		for dz := -3; dz <= 3; dz++ {
			isBorder := dx == -3 || dx == 3 || dz == -3 || dz == 3
			if isBorder {
				place(fx+dx, surfY, fz+dz, log)
			} else if dx == 0 {
				place(fx+dx, surfY, fz+dz, water)
			} else if dx == -1 || dx == 1 {
				// Farmland for stems
				place(fx+dx, surfY, fz+dz, farmland)
				place(fx+dx, surfY+1, fz+dz, stem|7) // fully grown stem
			} else {
				// Dirt-like foundation for fruit to spawn on
				// In villages, this is usually grass or dirt. We'll use grass.
				place(fx+dx, surfY, fz+dz, 2<<4) // grass
				// Randomly place a few fruits already
				if (dx+dz)%3 == 0 {
					place(fx+dx, surfY+1, fz+dz, fruit)
				}
			}
		}
	}
}

// buildLargeHall creates a 7x9 building with a solid symmetrical facade.
func (v *VillageGrid) buildLargeHall(hx, hz, surfY int, place func(wx, y, wz int, state uint16), log, planks, cobble, glass, torch, stairs, door, air, oakSlab, bed uint16) {
	const w, l = 7, 9
	const h = 4
	for dx := 0; dx < w; dx++ {
		for dz := 0; dz < l; dz++ {
			place(hx+dx, surfY, hz+dz, cobble) // foundation
			isCorner := (dx == 0 || dx == w-1) && (dz == 0 || dz == l-1)
			isWallX := dx == 0 || dx == w-1
			isWallZ := dz == 0 || dz == l-1
			isWall := isWallX || isWallZ

			for dy := 1; dy <= h; dy++ {
				if isCorner {
					place(hx+dx, surfY+dy, hz+dz, log)
				} else if isWall {
					// Default to planks
					block := planks

					// Symmetrical End Wall (Door Wall) at dz=l-1
					if dz == l-1 {
						isDoor := dx == 3 && (dy == 1 || dy == 2)
						isWindow := (dx == 1 || dx == 5) && dy == 2
						if isDoor {
							if dy == 1 {
								block = door | 1 // Bottom, North
							} else {
								block = door | 8 // Top
							}
						} else if isWindow {
							block = glass
						}
					} else if dz == 0 {
						// Back wall: symmetrical windows
						if (dx == 2 || dx == 4) && dy == 2 {
							block = glass
						}
					} else if isWallX {
						// Side walls: triple symmetrical windows
						if (dz == 2 || dz == 4 || dz == 6) && dy == 2 {
							block = glass
						}
					}
					place(hx+dx, surfY+dy, hz+dz, block)
				} else {
					place(hx+dx, surfY+dy, hz+dz, air)
				}
			}
		}
	}

	// Interior Furniture (Bed)
	// Place a bed in the back-left corner (hx+1, hz+1)
	// Foot at (hx+1, hz+2), Head at (hx+1, hz+1) facing North (-Z)
	place(hx+1, surfY+1, hz+2, bed|2)  // Foot
	place(hx+1, surfY+1, hz+1, bed|10) // Head

	// Pitched Roof (North-South oriented)
	for dz := -1; dz <= l; dz++ {
		// Slope coverage logic (w=7)
		place(hx-1, surfY+h+1, hz+dz, stairs|0) // Eaves
		place(hx+w, surfY+h+1, hz+dz, stairs|1)

		place(hx, surfY+h+2, hz+dz, stairs|0)
		place(hx+w-1, surfY+h+2, hz+dz, stairs|1)

		place(hx+1, surfY+h+3, hz+dz, stairs|0)
		place(hx+w-2, surfY+h+3, hz+dz, stairs|1)

		place(hx+2, surfY+h+4, hz+dz, stairs|0)
		place(hx+w-3, surfY+h+4, hz+dz, stairs|1)

		place(hx+3, surfY+h+5, hz+dz, oakSlab) // Ridge

		// Gables
		if dz == 0 || dz == l-1 {
			for dy := 1; dy <= 4; dy++ {
				for dx := 0; dx < w; dx++ {
					if (dy == 1) || (dy == 2 && dx >= 1 && dx <= 5) || (dy == 3 && dx >= 2 && dx <= 4) || (dy == 4 && dx == 3) {
						place(hx+dx, surfY+h+dy, hz+dz, planks)
					}
				}
			}
		}
	}
	place(hx+3, surfY+3, hz+l, torch)
}

// buildHouse places a 5x5 plank house with a solid symmetrical facade.
func (v *VillageGrid) buildHouse(hx, hz, surfY, houseIdx int, place func(wx, y, wz int, state uint16), log, planks, cobble, glass, fence, torch, stairs, door, air, oakSlab, bed uint16) {
	const w = 5
	const h = 3
	for dx := 0; dx < w; dx++ {
		for dz := 0; dz < w; dz++ {
			place(hx+dx, surfY, hz+dz, cobble)
			isCorner := (dx == 0 || dx == w-1) && (dz == 0 || dz == w-1)
			isWallX := dx == 0 || dx == w-1
			isWallZ := dz == 0 || dz == w-1
			isWall := isWallX || isWallZ

			for dy := 1; dy <= h; dy++ {
				if isCorner {
					place(hx+dx, surfY+dy, hz+dz, log)
				} else if isWall {
					block := planks

					// Door side depends on houseIdx
					isFrontWall := (houseIdx%2 == 0 && dz == w-1) || (houseIdx%2 == 1 && dz == 0)
					if isFrontWall {
						isDoor := dx == 2 && (dy == 1 || dy == 2)
						isWindow := (dx == 1 || dx == 3) && dy == 2
						if isDoor {
							meta := uint16(1) // North
							if houseIdx%2 == 1 {
								meta = 3 // South
							}
							if dy == 1 {
								block = door | meta
							} else {
								block = door | 8
							}
						} else if isWindow {
							block = glass
						}
					} else {
						// Other walls: centered window
						if dy == 2 {
							if (isWallX && dz == 2) || (isWallZ && dx == 2) {
								block = glass
							}
						}
					}
					place(hx+dx, surfY+dy, hz+dz, block)
				} else {
					place(hx+dx, surfY+dy, hz+dz, air)
				}
			}
		}
	}

	// Interior Furniture (Bed)
	// Bed orientation depends on entrance.
	if houseIdx%2 == 0 {
		// Door at Front (dz=w-1=4) facing North (-Z)
		// Place bed at back (dz=1, dz=2)
		place(hx+1, surfY+1, hz+2, bed|2)  // Foot
		place(hx+1, surfY+1, hz+1, bed|10) // Head
	} else {
		// Door at Front (dz=0) facing South (+Z)
		// Place bed at back (dz=3, dz=2)
		place(hx+3, surfY+1, hz+2, bed|0) // Foot
		place(hx+3, surfY+1, hz+3, bed|8) // Head
	}

	// Pitched Roof
	if houseIdx%2 == 0 {
		// North-South peak
		for dx := -1; dx <= w; dx++ {
			place(hx+dx, surfY+h+1, hz-1, stairs|2)
			place(hx+dx, surfY+h+1, hz+w, stairs|3)
			place(hx+dx, surfY+h+2, hz, stairs|2)
			place(hx+dx, surfY+h+2, hz+w-1, stairs|3)
			place(hx+dx, surfY+h+3, hz+1, stairs|2)
			place(hx+dx, surfY+h+3, hz+3, stairs|3)
			place(hx+dx, surfY+h+4, hz+2, oakSlab)

			if dx == 0 || dx == w-1 {
				for dy := 1; dy <= 3; dy++ {
					for dz := 1; dz < w-1; dz++ {
						if (dy == 1) || (dy == 2 && dz >= 1 && dz <= 3) || (dy == 3 && dz == 2) {
							place(hx+dx, surfY+h+dy, hz+dz, planks)
						}
					}
					place(hx+dx, surfY+h+1, hz, planks)
					place(hx+dx, surfY+h+1, hz+w-1, planks)
				}
			}
		}
	} else {
		// East-West peak
		for dz := -1; dz <= w; dz++ {
			place(hx-1, surfY+h+1, hz+dz, stairs|0)
			place(hx+w, surfY+h+1, hz+dz, stairs|1)
			place(hx, surfY+h+2, hz+dz, stairs|0)
			place(hx+w-1, surfY+h+2, hz+dz, stairs|1)
			place(hx+1, surfY+h+3, hz+dz, stairs|0)
			place(hx+3, surfY+h+3, hz+dz, stairs|1)
			place(hx+2, surfY+h+4, hz+dz, oakSlab)

			if dz == 0 || dz == w-1 {
				for dy := 1; dy <= 3; dy++ {
					for dx := 1; dx < w-1; dx++ {
						if (dy == 1) || (dy == 2 && dx >= 1 && dx <= 3) || (dy == 3 && dx == 2) {
							place(hx+dx, surfY+h+dy, hz+dz, planks)
						}
					}
					place(hx, surfY+h+1, hz+dz, planks)
					place(hx+w-1, surfY+h+1, hz+dz, planks)
				}
			}
		}
	}
}

// buildChurch creates a polished village church with a ground-level entrance and open belfry.
func (v *VillageGrid) buildChurch(hx, hz, surfY int, place func(wx, y, wz int, state uint16), log, planks, cobble, glass, torch, stairs, door, slab, air, bell uint16) {
	// Footprint split into two areas:
	// Tower: [0, 5) x [0, 5)
	// Hall:  [0, 5) x [4, 11) (Overlaps at dz=4)
	const towerMaxX, towerMaxZ = 4, 4
	const hallMaxX, hallMaxZ = 4, 10
	const hallHeight = 4
	const towerHeight = 10 // Solid part ends here

	// 1. Tower Construction
	for dx := 0; dx <= towerMaxX; dx++ {
		for dz := 0; dz <= towerMaxZ; dz++ {
			isWallX := dx == 0 || dx == towerMaxX
			isWallZ := dz == 0 || dz == towerMaxZ
			isCorner := isWallX && isWallZ

			for dy := -1; dy <= towerHeight+6; dy++ {
				if dy == -1 {
					place(hx+dx, surfY+dy, hz+dz, cobble) // Hidden Foundation
				} else if dy == 0 {
					place(hx+dx, surfY+dy, hz+dz, cobble) // Solid Floor at ground level
				} else if dy == towerHeight {
					place(hx+dx, surfY+dy, hz+dz, cobble) // Belfry Floor
				} else {
					// Corners go up to dy = towerHeight + 4 (Belfry Pillars)
					isBelfrySpace := dy > towerHeight && dy <= towerHeight+4
					if isCorner && dy <= towerHeight+4 {
						place(hx+dx, surfY+dy, hz+dz, log)
					} else if dy > towerHeight+4 {
						// Belfry Peaked Roof
						isRoofWallX := dx == 0 || dx == towerMaxX
						isRoofWallZ := dz == 0 || dz == towerMaxZ
						if dy == towerHeight+5 {
							if isRoofWallX || isRoofWallZ {
								place(hx+dx, surfY+dy, hz+dz, cobble)
							}
						} else if dy == towerHeight+6 {
							if dx >= 1 && dx <= 3 && dz >= 1 && dz <= 3 && (dx == 2 || dz == 2) {
								place(hx+dx, surfY+dy, hz+dz, cobble)
							}
							if dx == 2 && dz == 2 {
								place(hx+dx, surfY+dy+1, hz+dz, cobble) // Tip
							}
						}
					} else if !isBelfrySpace {
						// Solid Walls
						if isWallX || (isWallZ && (dy > hallHeight || dz == 0)) {
							block := cobble
							// Door and Cross at front
							if dz == 0 {
								if dx == 2 {
									if dy == 0 {
										block = door | 3
									} else if dy == 1 {
										block = door | 8
									} else if dy >= 4 && dy <= 6 {
										block = glass
									}
								} else if (dx == 1 || dx == 3) && dy == 5 {
									block = glass
								}
							}
							// Upper windows
							if dy == towerHeight-2 && (isWallX || dz == 0) {
								block = glass
							}
							place(hx+dx, surfY+dy, hz+dz, block)
						} else {
							if dy > 0 {
								place(hx+dx, surfY+dy, hz+dz, air) // Hollow interior
							}
						}
					} else {
						// Open belfry space
						if dy == towerHeight+4 && isWallZ && (dx == 1 || dx == 3) {
							place(hx+dx, surfY+dy, hz+dz, cobble) // Arch connectors
						} else if dy == towerHeight+4 && isWallX && (dz == 1 || dz == 3) {
							place(hx+dx, surfY+dy, hz+dz, cobble) // Arch connectors
						} else {
							place(hx+dx, surfY+dy, hz+dz, air)
						}
					}
				}
			}
		}
	}

	// 2. Hanging Bell
	place(hx+2, surfY+towerHeight+4, hz+2, 85<<4) // Fence hanging from belfry roof center
	place(hx+2, surfY+towerHeight+3, hz+2, bell)  // The Gold Bell

	// 3. Hall Construction
	for dx := 0; dx <= hallMaxX; dx++ {
		for dz := 5; dz <= hallMaxZ; dz++ {
			isWallX := dx == 0 || dx == hallMaxX
			isWallZ := dz == hallMaxZ
			isCorner := isWallX && (dz == hallMaxZ)

			for dy := -1; dy <= hallHeight; dy++ {
				if dy == -1 {
					place(hx+dx, surfY+dy, hz+dz, cobble) // Hidden Foundation
				} else if dy == 0 {
					place(hx+dx, surfY+dy, hz+dz, cobble) // Solid Floor
				} else if isCorner {
					place(hx+dx, surfY+dy, hz+dz, log) // Back corners
				} else if isWallX || isWallZ {
					block := cobble
					// Windows
					if dy == 1 || dy == 2 {
						if isWallX && (dz == 6 || dz == 8) {
							block = glass
						} else if isWallZ && dx == 2 {
							block = glass
						}
					}
					place(hx+dx, surfY+dy, hz+dz, block)
				} else {
					if dy > 0 {
						place(hx+dx, surfY+dy, hz+dz, air)
					}
				}
			}
		}
	}

	// 4. Junction Closure (dz=4)
	for dx := 1; dx < hallMaxX; dx++ {
		for dy := 1; dy <= hallHeight; dy++ {
			place(hx+dx, surfY+dy, hz+4, air)
		}
	}

	// 5. Hall Gables and Roof
	for dz := 4; dz <= hallMaxZ; dz++ {
		// Back Gable (dz=10)
		if dz == hallMaxZ {
			place(hx+1, surfY+hallHeight+1, hz+dz, cobble)
			place(hx+2, surfY+hallHeight+1, hz+dz, cobble)
			place(hx+3, surfY+hallHeight+1, hz+dz, cobble)
			place(hx+2, surfY+hallHeight+2, hz+dz, cobble)
		}

		// Slopes
		place(hx-1, surfY+hallHeight+1, hz+dz, stairs|0)
		place(hx+hallMaxX+1, surfY+hallHeight+1, hz+dz, stairs|1)

		place(hx, surfY+hallHeight+1, hz+dz, cobble)
		place(hx, surfY+hallHeight+2, hz+dz, stairs|0)

		place(hx+hallMaxX, surfY+hallHeight+1, hz+dz, cobble)
		place(hx+hallMaxX, surfY+hallHeight+2, hz+dz, stairs|1)

		place(hx+1, surfY+hallHeight+2, hz+dz, cobble)
		place(hx+1, surfY+hallHeight+3, hz+dz, stairs|0)

		place(hx+hallMaxX-1, surfY+hallHeight+2, hz+dz, cobble)
		place(hx+hallMaxX-1, surfY+hallHeight+3, hz+dz, stairs|1)

		// Ridge - Extends into the tower wall
		place(hx+2, surfY+hallHeight+3, hz+dz, cobble)
		place(hx+2, surfY+hallHeight+4, hz+dz, slab)
	}

	// 6. Interior Furniture
	// Fixed pews: 3 = North, 2 = South, 0 = East, 1 = West
	// We want them facing the altar (dz=hallMaxZ), which is North in our coordinate system?
	// Actually dz increases. Altar is at dz=9/10. So they should face North (3).
	// Let's re-verify:
	// dz = 0 is Front (Entrance)
	// dz = 11 is Back (Altar)
	// So facing "Back" is facing North.
	place(hx+1, surfY+1, hz+6, stairs|3)
	place(hx+3, surfY+1, hz+6, stairs|3)
	place(hx+1, surfY+1, hz+8, stairs|3)
	place(hx+3, surfY+1, hz+8, stairs|3)

	place(hx+2, surfY+1, hz+hallMaxZ-1, cobble) // Altar
	place(hx+2, surfY+2, hz+hallMaxZ-1, torch|5)
}
