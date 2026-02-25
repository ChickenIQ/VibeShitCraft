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
// (cellX, cellZ), and ok=true if that cell contains a village (25% chance)
// AND no neighboring village is too close (minimum 80 blocks apart).
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
	wx = cellX*villageCellSize + ox
	wz = cellZ*villageCellSize + oz

	// Check neighboring cells — suppress this village if a neighbor is too close.
	// Only yield to neighbors with a lower cell hash (deterministic tie-breaking).
	const minDist = 80
	myPriority := v.cellHash(cx^0x1234, cz^0x5678, 1000)
	for dx := -1; dx <= 1; dx++ {
		for dz := -1; dz <= 1; dz++ {
			if dx == 0 && dz == 0 {
				continue
			}
			ncx := int64(cellX + dx)
			ncz := int64(cellZ + dz)
			if v.cellHash(ncx, ncz, 4) != 0 {
				continue // neighbor cell has no village
			}
			nox := int(v.cellHash(ncx^0xDEAD, ncz^0xBEEF, int64(villageCellSize-20))) + 10
			noz := int(v.cellHash(ncx^0xCAFE, ncz^0xF00D, int64(villageCellSize-20))) + 10
			nwx := (cellX+dx)*villageCellSize + nox
			nwz := (cellZ+dz)*villageCellSize + noz

			ddx := wx - nwx
			ddz := wz - nwz
			if ddx < 0 {
				ddx = -ddx
			}
			if ddz < 0 {
				ddz = -ddz
			}
			if ddx+ddz < minDist {
				// Too close — suppress the one with higher priority value
				neighborPriority := v.cellHash(ncx^0x1234, ncz^0x5678, 1000)
				if myPriority >= neighborPriority {
					return 0, 0, false
				}
			}
		}
	}

	return wx, wz, true
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
	// Increased to 75 to capture longer branching paths and offset buildings.
	const radius = 75

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
	// Decreased from 75 to 55 to allow trees/structures to spawn closer to villages
	const influenceRadius = 55
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

// doorToAnchorHouse computes the building anchor (hx, hz) for a 5x5 house
// given the door's world position and facing direction.
// facing: 0=door on +Z wall, 1=door on -Z wall, 2=door on +X wall, 3=door on -X wall
func doorToAnchorHouse(doorX, doorZ, facing int) (int, int) {
	switch facing {
	case 0: // door at (hx+2, hz+4)
		return doorX - 2, doorZ - 4
	case 1: // door at (hx+2, hz)
		return doorX - 2, doorZ
	case 2: // door at (hx+4, hz+2)
		return doorX - 4, doorZ - 2
	case 3: // door at (hx, hz+2)
		return doorX, doorZ - 2
	}
	return doorX, doorZ
}

// doorToAnchorHall computes the building anchor for a 7x9 large hall.
// facing 0: door at (hx+3, hz+8); facing 1: door at (hx+3, hz)
func doorToAnchorHall(doorX, doorZ, facing int) (int, int) {
	switch facing {
	case 0: // door at (hx+3, hz+l-1=hz+8)
		return doorX - 3, doorZ - 8
	case 1: // door at (hx+3, hz)
		return doorX - 3, doorZ
	}
	return doorX, doorZ
}

// doorToAnchorChurch computes the building anchor for the church.
// facing 0: door at (hx+2, hz+10); facing 1: door at (hx+2, hz)
func doorToAnchorChurch(doorX, doorZ, facing int) (int, int) {
	switch facing {
	case 0: // door at (hx+2, hz+hallMaxZ=hz+10)
		return doorX - 2, doorZ - 10
	case 1: // door at (hx+2, hz)
		return doorX - 2, doorZ
	}
	return doorX, doorZ
}

// doorToAnchorMarketplace computes the anchor for the 13x13 marketplace.
// The anchor is the minimum x,z corner. The "door" is the entry to the central cross-path.
// facing 0: entry on +Z wall; facing 1: entry on -Z wall
func doorToAnchorMarketplace(doorX, doorZ, facing int) (int, int) {
	switch facing {
	case 0: // entry at (hx+6, hz+12)
		return doorX - 6, doorZ - 12
	case 1: // entry at (hx+6, hz)
		return doorX - 6, doorZ
	case 2: // entry at (hx+12, hz+6)
		return doorX - 12, doorZ - 6
	case 3: // entry at (hx, hz+6)
		return doorX, doorZ - 6
	}
	return doorX, doorZ
}

// roadSegment represents one axis-aligned road segment in the village road tree.
// Roads branch off each other: each segment can have child branches that fork
// perpendicular to the parent at some point along its length.
type roadSegment struct {
	// Start and end points (world coords). One of X or Z varies; the other is constant.
	sx, sz, ex, ez int
	// horizontal: true = varies along X (constant Z), false = varies along Z (constant X)
	horizontal bool
	// children forking off this segment
	children []roadSegment
	// wobbleSeed for gentle curvature
	wobbleSeed int
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
		oakSlab      = 126 << 4 // oak slab
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
		bed          = 26 << 4  // bed
	)

	abs := func(x int) int {
		if x < 0 {
			return -x
		}
		return x
	}

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
	// 1. Generate branching road tree
	// ------------------------------------------------------------------
	// Decide which main arms exist (2-4 directions from the well)
	armPosX := v.cellHash(int64(vx)^0xA1, int64(vz)^0xB2, 4) < 3
	armNegX := v.cellHash(int64(vx)^0xC3, int64(vz)^0xD4, 4) < 3
	armPosZ := v.cellHash(int64(vx)^0xE5, int64(vz)^0xF6, 4) < 3
	armNegZ := v.cellHash(int64(vx)^0x17, int64(vz)^0x28, 4) < 3

	armCount := 0
	if armPosX {
		armCount++
	}
	if armNegX {
		armCount++
	}
	if armPosZ {
		armCount++
	}
	if armNegZ {
		armCount++
	}
	if armCount < 2 {
		armPosX = true
		armNegZ = true
	}

	// We maintain a flat list of all generated segments to check for collisions/intersections.
	// This prevents roads from looping back onto each other or running too close parallel.
	var allSegmentsFlat []roadSegment

	// Helper to check if a proposed segment bounding box gets too close (within 12 blocks)
	// to any already generated segment, EXCEPT for the exact point where it forks from its parent.
	checkRoadCollision := func(psx, psz, pex, pez int, parentSx, parentSz int) bool {
		minX, maxX := psx, pex
		if minX > maxX {
			minX, maxX = maxX, minX
		}
		minZ, maxZ := psz, pez
		if minZ > maxZ {
			minZ, maxZ = maxZ, minZ
		}

		// Expand the proposed box by 12 blocks for clearance
		pMinX, pMaxX := minX-12, maxX+12
		pMinZ, pMaxZ := minZ-12, maxZ+12

		for _, exSeg := range allSegmentsFlat {
			eMinX, eMaxX := exSeg.sx, exSeg.ex
			if eMinX > eMaxX {
				eMinX, eMaxX = eMaxX, eMinX
			}
			eMinZ, eMaxZ := exSeg.sz, exSeg.ez
			if eMinZ > eMaxZ {
				eMinZ, eMaxZ = eMaxZ, eMinZ
			}

			// If bounding boxes overlap, it's a collision
			if pMinX <= eMaxX && pMaxX >= eMinX && pMinZ <= eMaxZ && pMaxZ >= eMinZ {
				// Exception: if the existing segment IS the parent segment we are branching from,
				// they will obviously intersect. We loosely check if the collision is just the fork point.
				// Since branches are perpendicular to parents, we just let it pass if this branch
				// starts exactly ON the existing segment's axis. We can check if parentSx, parentSz
				// falls ON the existing segment. (Simplification: if the branch origins lie on the exSeg, assume it's the parent).
				if parentSx >= eMinX && parentSx <= eMaxX && parentSz >= eMinZ && parentSz <= eMaxZ {
					continue
				}
				return true
			}
		}
		return false
	}

	// generateBranch creates a road segment with potential child branches.
	// depth limits recursion; salt provides deterministic variation.
	// returns a boolean indicating if the segment was successfully created (no collision).
	var generateBranch func(sx, sz int, horizontal bool, length, dir, depth, salt int, out *roadSegment) bool
	generateBranch = func(sx, sz int, horizontal bool, length, dir, depth, salt int, out *roadSegment) bool {
		var ex, ez int
		if horizontal {
			ex = sx + length*dir
			ez = sz
		} else {
			ex = sx
			ez = sz + length*dir
		}

		// Only check collision for depth < 2 (branches). Main roads (depth==2) don't collide with each other.
		if depth < 2 {
			if checkRoadCollision(sx, sz, ex, ez, sx, sz) {
				return false
			}
		}

		seg := roadSegment{
			sx: sx, sz: sz, ex: ex, ez: ez,
			horizontal: horizontal,
			wobbleSeed: salt,
		}

		// Commit to flat list before generating children so children can check against us
		allSegmentsFlat = append(allSegmentsFlat, seg)

		if depth <= 0 || length < 10 {
			*out = seg
			return true
		}

		// Try to spawn 1-2 branches along this segment
		numBranchAttempts := 1
		if length >= 20 {
			numBranchAttempts = 2
		}

		for b := 0; b < numBranchAttempts; b++ {
			branchSalt := salt*31 + b*17 + depth*7
			// ~60% chance of a branch
			if v.cellHash(int64(branchSalt)^0xBB, int64(sx+sz+b), 10) < 4 {
				continue
			}

			// Where along the segment does the branch fork?
			// Between 40%-80% of the way along
			frac := int(v.cellHash(int64(branchSalt)^0xCC, int64(ex+ez), 40)) + 40
			branchDist := length * frac / 100
			if branchDist < 8 {
				branchDist = 8
			}

			// Branch start point
			var bsx, bsz int
			if horizontal {
				bsx = sx + branchDist*dir
				bsz = sz
			} else {
				bsx = sx
				bsz = sz + branchDist*dir
			}

			// Branch goes perpendicular; pick direction (+1 or -1)
			branchDir := 1
			if v.cellHash(int64(branchSalt)^0xDD, int64(bsx+bsz), 2) == 0 {
				branchDir = -1
			}

			// Branch length: 14-28 blocks
			branchLen := int(v.cellHash(int64(branchSalt)^0xEE, int64(bsx-bsz), 15)) + 14

			var child roadSegment
			if generateBranch(bsx, bsz, !horizontal, branchLen, branchDir, depth-1, branchSalt+1000, &child) {
				seg.children = append(seg.children, child)
			}
		}

		*out = seg
		return true
	}

	// Main road lengths: 24-45 blocks
	mainLen := func(salt int) int {
		return int(v.cellHash(int64(vx)^int64(salt), int64(vz)^int64(salt*3), 22)) + 24
	}

	var allSegments []roadSegment
	if armPosX {
		var seg roadSegment
		generateBranch(vx, vz, true, mainLen(0x111), 1, 2, vx+1, &seg)
		allSegments = append(allSegments, seg)
	}
	if armNegX {
		var seg roadSegment
		generateBranch(vx, vz, true, mainLen(0x222), -1, 2, vx+2, &seg)
		allSegments = append(allSegments, seg)
	}
	if armPosZ {
		var seg roadSegment
		generateBranch(vx, vz, false, mainLen(0x333), 1, 2, vz+3, &seg)
		allSegments = append(allSegments, seg)
	}
	if armNegZ {
		var seg roadSegment
		generateBranch(vx, vz, false, mainLen(0x444), -1, 2, vz+4, &seg)
		allSegments = append(allSegments, seg)
	}

	// ------------------------------------------------------------------
	// 2. Collect building sites from the road tree
	// ------------------------------------------------------------------
	type buildingSite struct {
		doorX, doorZ int
		facing       int // 0=+Z, 1=-Z, 2=+X, 3=-X
		smallOnly    bool
	}

	var sites []buildingSite

	// collectSites walks the road tree and places buildings at branch endpoints
	// and midpoints of long segments.
	var collectSites func(seg *roadSegment, depth int)
	collectSites = func(seg *roadSegment, depth int) {
		segLen := abs(seg.ex-seg.sx) + abs(seg.ez-seg.sz)

		// Place a building at the far end of this segment (if long enough)
		if segLen >= 10 {
			// Building at the endpoint — door faces back toward the road
			// Pick which side of the road the building is on
			sideHash := v.cellHash(int64(seg.ex)^0xF1, int64(seg.ez)^0xF2, 2)
			sideOffset := 5
			if sideHash == 0 {
				sideOffset = -5
			}

			if seg.horizontal {
				// Road goes along X; place building to the +Z or -Z side
				facing := 0 // door on +Z wall (building is on -Z side of road)
				if sideOffset > 0 {
					facing = 1 // door on -Z wall (building is on +Z side of road)
				}
				sites = append(sites, buildingSite{
					doorX: seg.ex, doorZ: seg.ez + sideOffset,
					facing: facing, smallOnly: depth > 0,
				})
			} else {
				// Road goes along Z; place building to the +X or -X side
				facing := 2 // door on +X wall
				if sideOffset > 0 {
					facing = 3 // door on -X wall
				}
				sites = append(sites, buildingSite{
					doorX: seg.ex + sideOffset, doorZ: seg.ez,
					facing: facing, smallOnly: depth > 0,
				})
			}
		}

		// For longer segments, also place a building near the midpoint
		if segLen >= 22 {
			midHash := v.cellHash(int64(seg.sx+seg.ex)^0xA5, int64(seg.sz+seg.ez)^0xB6, 2)
			midSide := 5
			if midHash == 0 {
				midSide = -5
			}

			midX := (seg.sx + seg.ex) / 2
			midZ := (seg.sz + seg.ez) / 2

			if seg.horizontal {
				facing := 0
				if midSide > 0 {
					facing = 1
				}
				sites = append(sites, buildingSite{
					doorX: midX, doorZ: midZ + midSide,
					facing: facing, smallOnly: true,
				})
			} else {
				facing := 2
				if midSide > 0 {
					facing = 3
				}
				sites = append(sites, buildingSite{
					doorX: midX + midSide, doorZ: midZ,
					facing: facing, smallOnly: true,
				})
			}
		}

		// Recurse into children
		for i := range seg.children {
			collectSites(&seg.children[i], depth+1)
		}
	}

	for i := range allSegments {
		collectSites(&allSegments[i], 0)
	}

	// ------------------------------------------------------------------
	// 3. Decide building types for each site
	// ------------------------------------------------------------------
	type bbox struct{ minX, minZ, maxX, maxZ int }
	var placedBuildings []bbox

	type slotDecision struct {
		site       buildingSite
		structType int // 1=house, 2=hall, 3=church, 4=marketplace
	}
	var activeSlots []slotDecision
	hasHall := false
	hasChurch := false
	hasMarket := false

	// Collect road bounding boxes for building-vs-road collision
	var roadBBs []bbox
	var collectRoadBBs func(seg *roadSegment)
	collectRoadBBs = func(seg *roadSegment) {
		if seg.horizontal {
			minX, maxX := seg.sx, seg.ex
			if minX > maxX {
				minX, maxX = maxX, minX
			}
			roadBBs = append(roadBBs, bbox{minX, seg.sz - 2, maxX, seg.sz + 2})
		} else {
			minZ, maxZ := seg.sz, seg.ez
			if minZ > maxZ {
				minZ, maxZ = maxZ, minZ
			}
			roadBBs = append(roadBBs, bbox{seg.sx - 2, minZ, seg.sx + 2, maxZ})
		}
		for i := range seg.children {
			collectRoadBBs(&seg.children[i])
		}
	}
	for i := range allSegments {
		collectRoadBBs(&allSegments[i])
	}

	// We want to ensure at least 5 buildings spawn if possible.
	var fallbackSites []buildingSite

	for i, site := range sites {
		slotHash := v.cellHash(int64(site.doorX+i*7), int64(site.doorZ+i*13+3), 100)
		if slotHash < 20 {
			fallbackSites = append(fallbackSites, site)
			continue // ~20% chance to skip — not every site gets a building initially
		}

		structType := 1
		if !site.smallOnly {
			if slotHash >= 90 && !hasHall {
				structType = 2
			} else if slotHash >= 80 && !hasChurch {
				structType = 3
			} else if slotHash >= 70 && !hasMarket {
				structType = 4
			}
		}

		// Compute bounding box for collision check
		doorX, doorZ := site.doorX, site.doorZ
		var bx, bz, ex, ez int
		switch structType {
		case 1:
			hx, hz := doorToAnchorHouse(doorX, doorZ, site.facing)
			bx, bz, ex, ez = hx-1, hz-1, hx+6, hz+6
		case 2:
			hx, hz := doorToAnchorHall(doorX, doorZ, site.facing)
			bx, bz, ex, ez = hx-1, hz-1, hx+8, hz+10
		case 3:
			hx, hz := doorToAnchorChurch(doorX, doorZ, site.facing)
			bx, bz, ex, ez = hx-1, hz-1, hx+6, hz+12
		case 4:
			hx, hz := doorToAnchorMarketplace(doorX, doorZ, site.facing)
			bx, bz, ex, ez = hx-1, hz-1, hx+14, hz+14 // 13x13 footprint
		}

		// Check for collision with already-placed buildings AND road segments
		newBB := bbox{bx, bz, ex, ez}
		collides := false
		for _, bb := range placedBuildings {
			if newBB.minX <= bb.maxX && newBB.maxX >= bb.minX &&
				newBB.minZ <= bb.maxZ && newBB.maxZ >= bb.minZ {
				collides = true
				break
			}
		}
		if !collides {
			for _, rbb := range roadBBs {
				if newBB.minX <= rbb.maxX && newBB.maxX >= rbb.minX &&
					newBB.minZ <= rbb.maxZ && newBB.maxZ >= rbb.minZ {
					collides = true
					break
				}
			}
		}
		if collides {
			fallbackSites = append(fallbackSites, site)
			continue
		}

		// Mark unique buildings as spawned since they passed collision
		if structType == 2 {
			hasHall = true
		} else if structType == 3 {
			hasChurch = true
		} else if structType == 4 {
			hasMarket = true
		}

		placedBuildings = append(placedBuildings, newBB)
		activeSlots = append(activeSlots, slotDecision{site, structType})
	}

	// Rescue loop: if we have fewer than 5 buildings, aggressively try the fallback sites
	for _, site := range fallbackSites {
		if len(placedBuildings) >= 5 {
			break
		}

		structType := 1 // Only rescue basic houses
		doorX, doorZ := site.doorX, site.doorZ
		hx, hz := doorToAnchorHouse(doorX, doorZ, site.facing)
		bx, bz, ex, ez := hx-1, hz-1, hx+6, hz+6

		newBB := bbox{bx, bz, ex, ez}
		collides := false
		for _, bb := range placedBuildings {
			if newBB.minX <= bb.maxX && newBB.maxX >= bb.minX &&
				newBB.minZ <= bb.maxZ && newBB.maxZ >= bb.minZ {
				collides = true
				break
			}
		}
		if !collides {
			for _, rbb := range roadBBs {
				if newBB.minX <= rbb.maxX && newBB.maxX >= rbb.minX &&
					newBB.minZ <= rbb.maxZ && newBB.maxZ >= rbb.minZ {
					collides = true
					break
				}
			}
		}
		if !collides {
			placedBuildings = append(placedBuildings, newBB)
			activeSlots = append(activeSlots, slotDecision{site, structType})
		}
	}

	// ------------------------------------------------------------------
	// 4. Place farms (same locations, collision-checked)
	// ------------------------------------------------------------------
	farmLocations := []struct{ dx, dz int }{
		{-24, 14},
		{8, -26},
		{24, 14},
		{-8, -32},
		{30, -14},
		{-28, -16},
	}
	numFarms := int(v.cellHash(int64(vx), int64(vz), 4)) + 1
	if numFarms > len(farmLocations) {
		numFarms = len(farmLocations)
	}
	for i := len(farmLocations) - 1; i > 0; i-- {
		j := int(v.cellHash(int64(vx+i), int64(vz-i), int64(i+1)))
		farmLocations[i], farmLocations[j] = farmLocations[j], farmLocations[i]
	}

	type placedFarm struct{ dx, dz int }
	var activeFarms []placedFarm

	// Ensure at least 1 farm is placed if possible
	targetFarms := numFarms
	if targetFarms < 1 {
		targetFarms = 1
	}

	for i := 0; i < len(farmLocations); i++ {
		if len(activeFarms) >= targetFarms {
			break
		}
		fdx, fdz := farmLocations[i].dx, farmLocations[i].dz
		fx, fz := vx+fdx, vz+fdz
		farmBB := bbox{fx - 4, fz - 4, fx + 4, fz + 4}
		collides := false
		for _, bb := range placedBuildings {
			if farmBB.minX <= bb.maxX && farmBB.maxX >= bb.minX &&
				farmBB.minZ <= bb.maxZ && farmBB.maxZ >= bb.minZ {
				collides = true
				break
			}
		}
		if !collides {
			for _, rbb := range roadBBs {
				if farmBB.minX <= rbb.maxX && farmBB.maxX >= rbb.minX &&
					farmBB.minZ <= rbb.maxZ && farmBB.maxZ >= rbb.minZ {
					collides = true
					break
				}
			}
		}
		if !collides {
			activeFarms = append(activeFarms, placedFarm{fdx, fdz})
		}
	}

	// ------------------------------------------------------------------
	// 5. Prune road branches that don't lead to any building
	// ------------------------------------------------------------------
	// Build a set of placed building door positions for quick lookup
	type point struct{ x, z int }
	buildingDoors := make(map[point]bool)
	for _, sd := range activeSlots {
		buildingDoors[point{sd.site.doorX, sd.site.doorZ}] = true
	}

	// pruneRoad removes or shortens child branches that extend past the last building/farm.
	// Returns true if this segment itself should be kept (it has a building/farm or kept children).
	var pruneRoad func(seg *roadSegment, isMain bool) bool
	pruneRoad = func(seg *roadSegment, isMain bool) bool {
		// First, recurse into children and prune them
		kept := seg.children[:0]
		for i := range seg.children {
			if pruneRoad(&seg.children[i], false) {
				kept = append(kept, seg.children[i])
			}
		}
		seg.children = kept

		// We need to find the furthest point along this segment's axis that is actually needed.
		// A segment is needed up to the coordinate of:
		// - Its furthest surviving child branch
		// - Its furthest served building door
		// - Its furthest served farm center

		// Start with the origin of the segment as the "min required distance"
		maxDist := 0

		// Helper to check if a point (px, pz) needs this segment (is within 12 blocks of it perpendicularly)
		// and update maxDist if so.
		checkPoint := func(px, pz int) {
			if seg.horizontal {
				// Seg goes along X (constant Z)
				// Is it near our Z-axis?
				if abs(pz-seg.sz) <= 12 {
					// Is it along our X-span?
					dist := px - seg.sx
					if seg.ex < seg.sx {
						dist = seg.sx - px // going negative X
					}
					if dist >= -4 { // a little backwards leniency for off-center doors
						if dist > maxDist {
							maxDist = dist
						}
					}
				}
			} else {
				// Seg goes along Z (constant X)
				if abs(px-seg.sx) <= 12 {
					dist := pz - seg.sz
					if seg.ez < seg.sz {
						dist = seg.sz - pz // going negative Z
					}
					if dist >= -4 {
						if dist > maxDist {
							maxDist = dist
						}
					}
				}
			}
		}

		// 1. Children
		for _, child := range seg.children {
			checkPoint(child.sx, child.sz)
		}

		// 2. Buildings
		for _, sd := range activeSlots {
			checkPoint(sd.site.doorX, sd.site.doorZ)
		}

		// 3. Farms
		for _, f := range activeFarms {
			checkPoint(vx+f.dx, vz+f.dz)
		}

		// Original intended length
		originalLen := abs(seg.ex - seg.sx)
		if !seg.horizontal {
			originalLen = abs(seg.ez - seg.sz)
		}

		// If maxDist is 0 and we aren't a main road with children, we can kill this branch.
		// (Main roads from well are allowed to be 3 blocks long just to have a tiny stub if we want,
		// but let's prune completely empty ones too.)
		if maxDist <= 0 && len(seg.children) == 0 {
			// Actually, if it's a main road, give it a tiny 3-block stub so the well doesn't awkwardly end
			if isMain {
				maxDist = 3
			} else {
				return false
			}
		}

		// Truncate segment if maxDist is less than intended length
		// Add +2 to the required distance so the road extends slightly past the building door
		newLen := maxDist + 2
		if newLen > originalLen {
			newLen = originalLen
		}

		if seg.horizontal {
			if seg.ex < seg.sx {
				seg.ex = seg.sx - newLen
			} else {
				seg.ex = seg.sx + newLen
			}
		} else {
			if seg.ez < seg.sz {
				seg.ez = seg.sz - newLen
			} else {
				seg.ez = seg.sz + newLen
			}
		}

		return true
	}

	for i := range allSegments {
		pruneRoad(&allSegments[i], true)
	}

	// ------------------------------------------------------------------
	// 6. Render all road segments (walk tree, place 3-wide gravel with wobble)
	// ------------------------------------------------------------------
	wobble := func(d, seed int) int {
		drift := 0
		for seg := 1; seg <= d/16; seg++ {
			h := v.cellHash(int64(seed), int64(seg), 10)
			if h < 3 {
				drift--
			} else if h >= 7 {
				drift++
			}
		}
		if drift > 2 {
			drift = 2
		} else if drift < -2 {
			drift = -2
		}
		return drift
	}

	var renderRoad func(seg *roadSegment)
	renderRoad = func(seg *roadSegment) {
		if seg.horizontal {
			startX, endX := seg.sx, seg.ex
			if startX > endX {
				startX, endX = endX, startX
			}
			for x := startX; x <= endX; x++ {
				d := abs(x - seg.sx)
				wo := wobble(d, seg.wobbleSeed)
				for w := -1; w <= 1; w++ {
					placePath(x, surfY, seg.sz+w+wo, gravel)
				}
			}
		} else {
			startZ, endZ := seg.sz, seg.ez
			if startZ > endZ {
				startZ, endZ = endZ, startZ
			}
			for z := startZ; z <= endZ; z++ {
				d := abs(z - seg.sz)
				wo := wobble(d, seg.wobbleSeed)
				for w := -1; w <= 1; w++ {
					placePath(seg.sx+w+wo, surfY, z, gravel)
				}
			}
		}
		for i := range seg.children {
			renderRoad(&seg.children[i])
		}
	}

	for i := range allSegments {
		renderRoad(&allSegments[i])
	}

	// ------------------------------------------------------------------
	// 6. Well (at center)
	// ------------------------------------------------------------------
	for dx := -3; dx <= 2; dx++ {
		for dz := -3; dz <= 2; dz++ {
			placePath(vx+dx, surfY, vz+dz, gravel)
		}
	}
	for dx := -2; dx <= 1; dx++ {
		for dz := -2; dz <= 1; dz++ {
			isCorner := (dx == -2 || dx == 1) && (dz == -2 || dz == 1)
			isInner := (dx == -1 || dx == 0) && (dz == -1 || dz == 0)
			placeBlock(vx+dx, surfY+1, vz+dz, stoneBrick)
			if isCorner {
				placeBlock(vx+dx, surfY+2, vz+dz, cobbleWall)
				placeBlock(vx+dx, surfY+3, vz+dz, cobbleWall)
				placeBlock(vx+dx, surfY+4, vz+dz, cobbleWall)
			} else if isInner {
				placeBlock(vx+dx, surfY+1, vz+dz, stoneBrick)
				placeBlock(vx+dx, surfY+2, vz+dz, waterSrc)
			} else {
				placeBlock(vx+dx, surfY+2, vz+dz, stoneBrick)
			}
			placeBlock(vx+dx, surfY+5, vz+dz, slab)
		}
	}

	// ------------------------------------------------------------------
	// 7. Place buildings at decided sites
	// ------------------------------------------------------------------
	for _, sd := range activeSlots {
		doorX := sd.site.doorX
		doorZ := sd.site.doorZ

		switch sd.structType {
		case 1:
			hx, hz := doorToAnchorHouse(doorX, doorZ, sd.site.facing)
			v.buildHouse(hx, hz, surfY, sd.site.facing, placeBlock, log, planks, cobble, glass, fence, torch, oakStairs, woodenDoor, air, oakSlab, bed)
		case 2:
			hx, hz := doorToAnchorHall(doorX, doorZ, sd.site.facing)
			v.buildLargeHall(hx, hz, surfY, sd.site.facing, placeBlock, log, planks, cobble, glass, torch, oakStairs, woodenDoor, air, oakSlab, bed)
		case 3:
			hx, hz := doorToAnchorChurch(doorX, doorZ, sd.site.facing)
			v.buildChurch(hx, hz, surfY, sd.site.facing, placeBlock, log, planks, cobble, glass, torch, oakStairs, woodenDoor, slab, air, goldBlock)
		case 4:
			hx, hz := doorToAnchorMarketplace(doorX, doorZ, sd.site.facing)
			v.buildMarketplace(hx, hz, surfY, sd.site.facing, placeBlock, log, fence, oakSlab, gravel, torch, air)
		}

		// Gravel path from door to nearest road point (3 blocks wide)
		pathWidth := 1
		switch sd.site.facing {
		case 0:
			for z := doorZ; z <= doorZ+6; z++ {
				for w := -pathWidth; w <= pathWidth; w++ {
					placePath(doorX+w, surfY, z, gravel)
				}
			}
		case 1:
			for z := doorZ - 6; z <= doorZ; z++ {
				for w := -pathWidth; w <= pathWidth; w++ {
					placePath(doorX+w, surfY, z, gravel)
				}
			}
		case 2:
			for x := doorX; x <= doorX+6; x++ {
				for w := -pathWidth; w <= pathWidth; w++ {
					placePath(x, surfY, doorZ+w, gravel)
				}
			}
		case 3:
			for x := doorX - 6; x <= doorX; x++ {
				for w := -pathWidth; w <= pathWidth; w++ {
					placePath(x, surfY, doorZ+w, gravel)
				}
			}
		}
	}

	// ------------------------------------------------------------------
	// 8. Place farms
	// ------------------------------------------------------------------
	for _, f := range activeFarms {
		fx, fz := vx+f.dx, vz+f.dz
		farmHash := v.cellHash(int64(fx), int64(fz), 10)

		if farmHash < 7 {
			crops := []uint16{wheat, carrot, potato}
			v.buildFarm(fx, fz, surfY, placeBlock, log, waterSrc, farmland, crops[farmHash%3])
		} else {
			fruits := []uint16{melonBlock, pumpkinBlock}
			stems := []uint16{melonStem, pumpkinStem}
			idx := int(farmHash % 2)
			v.buildFruitFarm(fx, fz, surfY, placeBlock, log, waterSrc, farmland, stems[idx], fruits[idx])
		}
	}

	// ------------------------------------------------------------------
	// 9. Decorations: Flowers & Lamp Posts along road segments
	// ------------------------------------------------------------------
	for i := -40; i <= 40; i += 7 {
		for j := -40; j <= 40; j += 7 {
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

	// Lamp posts along all road segments at regular intervals
	var placeLamps func(seg *roadSegment)
	placeLamps = func(seg *roadSegment) {
		segLen := abs(seg.ex-seg.sx) + abs(seg.ez-seg.sz)

		for dist := 8; dist <= segLen-2; dist += 10 {
			var px, pz int
			if seg.horizontal {
				dir := 1
				if seg.ex < seg.sx {
					dir = -1
				}
				px = seg.sx + dist*dir
				pz = seg.sz + 2
			} else {
				dir := 1
				if seg.ez < seg.sz {
					dir = -1
				}
				px = seg.sx + 2
				pz = seg.sz + dist*dir
			}
			placeDecoration(px, surfY+1, pz, fence)
			placeDecoration(px, surfY+2, pz, fence)
			placeDecoration(px, surfY+3, pz, fence)
			placeDecoration(px, surfY+4, pz, torch|5)
		}

		for i := range seg.children {
			placeLamps(&seg.children[i])
		}
	}

	for i := range allSegments {
		placeLamps(&allSegments[i])
	}
}

// buildMarketplace creates a 13x13 open market with 4 log-and-slab stalls and a gravel cross in the middle.
func (v *VillageGrid) buildMarketplace(hx, hz, surfY, facing int, place func(wx, y, wz int, state uint16), log, fence, oakSlab, gravel, torch, air uint16) {
	// 13x13 bounding box. Clear footprint first.
	for dx := 0; dx < 13; dx++ {
		for dz := 0; dz < 13; dz++ {
			for dy := 1; dy <= 4; dy++ {
				place(hx+dx, surfY+dy, hz+dz, air)
			}
		}
	}

	// 1. Draw the gravel footprint
	// Instead of a cross, draw a solid rectangle that covers the stalls and the center path.
	if facing == 0 || facing == 1 {
		// Vertical layout. Gravel from X=2..10, Z=0..12
		for dx := 2; dx <= 10; dx++ {
			for dz := 0; dz <= 12; dz++ {
				place(hx+dx, surfY, hz+dz, gravel)
			}
		}
	} else {
		// Horizontal layout. Gravel from X=0..12, Z=2..10
		for dx := 0; dx <= 12; dx++ {
			for dz := 2; dz <= 10; dz++ {
				place(hx+dx, surfY, hz+dz, gravel)
			}
		}
	}

	upperSlab := oakSlab | 8 // meta 8 to make it upper slab

	// Function to generate a dynamic stall facing the specified axis
	buildStall := func(startX, startZ, w, l, frontDx, frontDz int) {
		for dx := 0; dx < w; dx++ {
			for dz := 0; dz < l; dz++ {
				// Logs at corners
				isCorner := (dx == 0 || dx == w-1) && (dz == 0 || dz == l-1)

				if isCorner {
					place(hx+startX+dx, surfY+1, hz+startZ+dz, log)
					place(hx+startX+dx, surfY+2, hz+startZ+dz, fence)
					place(hx+startX+dx, surfY+3, hz+startZ+dz, fence)
				}

				// Roof (covers entire stall)
				place(hx+startX+dx, surfY+4, hz+startZ+dz, oakSlab)

				// Counter (upper slab). Forms a U-shape open at the back.
				if !isCorner {
					isFront := false
					if frontDx >= 0 && dx == frontDx {
						isFront = true
					}
					if frontDz >= 0 && dz == frontDz {
						isFront = true
					}

					isEdge := false
					if frontDx >= 0 { // Stall faces along X-axis
						if dz == 0 || dz == l-1 {
							isEdge = true
						}
					} else if frontDz >= 0 { // Stall faces along Z-axis
						if dx == 0 || dx == w-1 {
							isEdge = true
						}
					}

					if isFront || isEdge {
						place(hx+startX+dx, surfY+1, hz+startZ+dz, upperSlab)
					}
				}
			}
		}
	}

	if facing == 0 || facing == 1 {
		// Main road enters along Z-axis. Center path is X=5..7.
		// Stalls face the vertical path (X=5..7).
		// Stalls are 3 wide (X) by 5 long (Z).
		buildStall(2, 0, 3, 5, 2, -1) // Top-Left, front facing +X (dx=2)
		buildStall(8, 0, 3, 5, 0, -1) // Top-Right, front facing -X (dx=0)
		buildStall(2, 8, 3, 5, 2, -1) // Bottom-Left, front facing +X (dx=2)
		buildStall(8, 8, 3, 5, 0, -1) // Bottom-Right, front facing -X (dx=0)
	} else {
		// Main road enters along X-axis. Center path is Z=5..7.
		// Stalls face the horizontal path (Z=5..7).
		// Stalls are 5 wide (X) by 3 long (Z).
		buildStall(0, 2, 5, 3, -1, 2) // Top-Left, front facing +Z (dz=2)
		buildStall(8, 2, 5, 3, -1, 2) // Top-Right, front facing +Z (dz=2)
		buildStall(0, 8, 5, 3, -1, 0) // Bottom-Left, front facing -Z (dz=0)
		buildStall(8, 8, 5, 3, -1, 0) // Bottom-Right, front facing -Z (dz=0)
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
// facing: 0=door on +Z wall, 1=door on -Z wall
func (v *VillageGrid) buildLargeHall(hx, hz, surfY, facing int, place func(wx, y, wz int, state uint16), log, planks, cobble, glass, torch, stairs, door, air, oakSlab, bed uint16) {
	const w, l = 7, 9
	const h = 4

	// Door and back wall Z positions depend on facing
	doorZ := l - 1 // facing=0: door on +Z
	backZ := 0
	doorMeta := uint16(1)
	if facing == 1 {
		doorZ = 0 // facing=1: door on -Z
		backZ = l - 1
		doorMeta = 3
	}

	for dx := 0; dx < w; dx++ {
		for dz := 0; dz < l; dz++ {
			place(hx+dx, surfY, hz+dz, cobble)
			isCorner := (dx == 0 || dx == w-1) && (dz == 0 || dz == l-1)
			isWallX := dx == 0 || dx == w-1
			isWallZ := dz == 0 || dz == l-1
			isWall := isWallX || isWallZ

			for dy := 1; dy <= h; dy++ {
				if isCorner {
					place(hx+dx, surfY+dy, hz+dz, log)
				} else if isWall {
					block := planks

					if dz == doorZ {
						isDoor := dx == 3 && (dy == 1 || dy == 2)
						isWindow := (dx == 1 || dx == 5) && dy == 2
						if isDoor {
							if dy == 1 {
								block = door | doorMeta
							} else {
								block = door | 8
							}
						} else if isWindow {
							block = glass
						}
					} else if dz == backZ {
						if (dx == 2 || dx == 4) && dy == 2 {
							block = glass
						}
					} else if isWallX {
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

	// Interior Furniture (Bed) - placed at the back of the hall
	if facing == 0 {
		place(hx+1, surfY+1, hz+2, bed|2)
		place(hx+1, surfY+1, hz+1, bed|10)
	} else {
		place(hx+1, surfY+1, hz+l-3, bed|0)
		place(hx+1, surfY+1, hz+l-2, bed|8)
	}

	// Pitched Roof
	for dz := -1; dz <= l; dz++ {
		place(hx-1, surfY+h+1, hz+dz, stairs|0)
		place(hx+w, surfY+h+1, hz+dz, stairs|1)

		place(hx, surfY+h+2, hz+dz, stairs|0)
		place(hx+w-1, surfY+h+2, hz+dz, stairs|1)

		place(hx+1, surfY+h+3, hz+dz, stairs|0)
		place(hx+w-2, surfY+h+3, hz+dz, stairs|1)

		place(hx+2, surfY+h+4, hz+dz, stairs|0)
		place(hx+w-3, surfY+h+4, hz+dz, stairs|1)

		place(hx+3, surfY+h+5, hz+dz, oakSlab)

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
	// Torch above door
	if facing == 0 {
		place(hx+3, surfY+3, hz+l, torch)
	} else {
		place(hx+3, surfY+3, hz-1, torch)
	}
}

// buildHouse places a 5x5 plank house with a solid symmetrical facade.
// facing: 0=door on +Z wall, 1=door on -Z wall, 2=door on +X wall, 3=door on -X wall
func (v *VillageGrid) buildHouse(hx, hz, surfY, facing int, place func(wx, y, wz int, state uint16), log, planks, cobble, glass, fence, torch, stairs, door, air, oakSlab, bed uint16) {
	const w = 5
	const h = 3

	// Door metadata per facing direction
	doorMetas := [4]uint16{1, 3, 2, 0} // +Z, -Z, +X, -X
	doorMeta := doorMetas[facing]

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

					// Determine front wall based on facing
					var isFrontWall bool
					switch facing {
					case 0:
						isFrontWall = dz == w-1
					case 1:
						isFrontWall = dz == 0
					case 2:
						isFrontWall = dx == w-1
					case 3:
						isFrontWall = dx == 0
					}

					if isFrontWall {
						var isDoor, isWindow bool
						switch facing {
						case 0, 1: // door on Z wall, centered on X
							isDoor = dx == 2 && (dy == 1 || dy == 2)
							isWindow = (dx == 1 || dx == 3) && dy == 2
						case 2, 3: // door on X wall, centered on Z
							isDoor = dz == 2 && (dy == 1 || dy == 2)
							isWindow = (dz == 1 || dz == 3) && dy == 2
						}
						if isDoor {
							if dy == 1 {
								block = door | doorMeta
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

	// Interior Furniture (Bed) - placed opposite the door
	switch facing {
	case 0: // door at +Z, bed at back (low Z)
		place(hx+1, surfY+1, hz+2, bed|2)
		place(hx+1, surfY+1, hz+1, bed|10)
	case 1: // door at -Z, bed at back (high Z)
		place(hx+3, surfY+1, hz+2, bed|0)
		place(hx+3, surfY+1, hz+3, bed|8)
	case 2: // door at +X, bed at back (low X)
		place(hx+2, surfY+1, hz+1, bed|1)
		place(hx+1, surfY+1, hz+1, bed|9)
	case 3: // door at -X, bed at back (high X)
		place(hx+2, surfY+1, hz+3, bed|3)
		place(hx+3, surfY+1, hz+3, bed|11)
	}

	// Pitched Roof - ridge perpendicular to door wall for variety
	if facing == 0 || facing == 1 {
		// Ridge runs along X axis, slopes face N/S
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
		// Ridge runs along Z axis, slopes face E/W
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
// facing: 0=door on +Z wall, 1=door on -Z wall.
// For facing=0 the entire structure is mirrored along Z.
func (v *VillageGrid) buildChurch(hx, hz, surfY, facing int, place func(wx, y, wz int, state uint16), log, planks, cobble, glass, torch, stairs, door, slab, air, bell uint16) {
	// For facing=0, mirror the Z axis. The church extends 11 blocks in Z (0..10).
	// We mirror: dz -> 10 - dz, and swap door meta and stair orientations.
	const totalZ = 10 // hallMaxZ
	actualPlace := place
	if facing == 0 {
		place = func(wx, y, wz int, state uint16) {
			// Mirror Z around center of church
			dz := wz - hz
			mirroredZ := hz + totalZ - dz
			// Rotate block metadata for doors and stairs
			blockID := state >> 4
			meta := state & 0xF
			if blockID == 64 { // wooden door
				if meta == 3 {
					meta = 1 // south -> north
				} else if meta == 1 {
					meta = 3
				}
				state = (blockID << 4) | meta
			} else if blockID == 67 { // cobblestone stairs
				if meta == 2 {
					meta = 3 // south -> north
				} else if meta == 3 {
					meta = 2
				}
				state = (blockID << 4) | meta
			} else if blockID == 53 { // oak stairs
				if meta == 2 {
					meta = 3
				} else if meta == 3 {
					meta = 2
				}
				state = (blockID << 4) | meta
			}
			actualPlace(wx, y, mirroredZ, state)
		}
	}
	// Footprint split into two areas:
	// Tower: [0, 5) x [0, 5)
	// Hall:  [0, 5) x [4, 11) (Overlaps at dz=4)
	const towerMaxX, towerMaxZ = 4, 4
	const hallMaxX, hallMaxZ = 4, 10
	const hallHeight = 4
	const towerHeight = 10 // Solid part ends here

	// Raise the whole church one block so it sits on the surface instead of sunk.
	surfY++

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
					// Place floor, but let corner logs extend down to touch the ground.
					if isCorner {
						place(hx+dx, surfY+dy, hz+dz, log)
					} else {
						place(hx+dx, surfY+dy, hz+dz, cobble) // Solid Floor at ground level
					}
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
									if dy == 1 {
										block = door | 3
									} else if dy == 2 {
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
	place(hx+2, surfY+towerHeight+5, hz+2, 85<<4) // Extra fence to attach to roof
	place(hx+2, surfY+towerHeight+4, hz+2, 85<<4) // Fence hanging from belfry roof center
	place(hx+2, surfY+towerHeight+3, hz+2, bell)  // The Gold Bell

	// Cobblestone stair in front of door so entrance is accessible.
	cobbleStairs := uint16(67 << 4) // stone/cobblestone stairs
	place(hx+2, surfY, hz-1, cobbleStairs|2)

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
					// Place floor; allow back-corner logs to start at floor level so they
					// touch the ground instead of floating.
					if isCorner {
						place(hx+dx, surfY+dy, hz+dz, log)
					} else {
						place(hx+dx, surfY+dy, hz+dz, cobble) // Solid Floor
					}
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
	// Helper to avoid placing roof blocks where the hall roof would intrude
	// into the tower footprint at dz==4 (overlap column). If a roof block
	// would land inside the tower (tz == hz+4 and tx in [hx, hx+towerMaxX])
	// skip it.
	placeRoof := func(tx, y, tz int, state uint16) {
		if tz == hz+4 {
			if tx >= hx && tx <= hx+towerMaxX {
				return
			}
		}
		place(tx, y, tz, state)
	}

	for dz := 4; dz <= hallMaxZ; dz++ {
		// Back Gable (dz=10)
		if dz == hallMaxZ {
			place(hx+1, surfY+hallHeight+1, hz+dz, cobble)
			place(hx+2, surfY+hallHeight+1, hz+dz, cobble)
			place(hx+3, surfY+hallHeight+1, hz+dz, cobble)
			place(hx+2, surfY+hallHeight+2, hz+dz, cobble)
		}

		// Slopes
		placeRoof(hx-1, surfY+hallHeight+1, hz+dz, stairs|0)
		placeRoof(hx+hallMaxX+1, surfY+hallHeight+1, hz+dz, stairs|1)

		placeRoof(hx, surfY+hallHeight+1, hz+dz, cobble)
		placeRoof(hx, surfY+hallHeight+2, hz+dz, stairs|0)

		placeRoof(hx+hallMaxX, surfY+hallHeight+1, hz+dz, cobble)
		placeRoof(hx+hallMaxX, surfY+hallHeight+2, hz+dz, stairs|1)

		placeRoof(hx+1, surfY+hallHeight+2, hz+dz, cobble)
		placeRoof(hx+1, surfY+hallHeight+3, hz+dz, stairs|0)

		placeRoof(hx+hallMaxX-1, surfY+hallHeight+2, hz+dz, cobble)
		placeRoof(hx+hallMaxX-1, surfY+hallHeight+3, hz+dz, stairs|1)

		// Ridge - Extends into the tower wall (skip overlap)
		placeRoof(hx+2, surfY+hallHeight+3, hz+dz, cobble)
		placeRoof(hx+2, surfY+hallHeight+4, hz+dz, slab)
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
