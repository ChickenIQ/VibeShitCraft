package server

import (
	"bytes"
	"log"
	"math"
	"math/rand"

	"github.com/VibeShit/VibeShitCraft/pkg/protocol"
	"github.com/VibeShit/VibeShitCraft/pkg/world"
)

func (s *Server) handleBlockBreak(player *Player, x, y, z int32) {
	blockState := s.world.GetBlock(x, y, z)
	blockID := blockState >> 4

	// Don't break air or bedrock
	if blockID == 0 || blockID == 7 {
		return
	}

	// Broadcast block break effect *before* changing the block state so that
	// the client still sees the correct block at this position when rendering
	// particles/sound. Sending the effect after turning the block into air
	// can cause client-side crashes for certain blocks (like stairs) that
	// expect specific properties on the block state.
	s.broadcastBlockBreakEffect(player, x, y, z, blockState)

	// In creative mode, don't give items on break
	var giveItem bool
	var itemID int16
	var damage int16
	var count byte

	if player.GameMode != GameModeCreative {
		giveItem = true
		itemID, damage, count = world.BlockToItemID(blockState)
	} else {
		log.Printf("Player %s broke block %d at (%d, %d, %d) (creative)", player.Username, blockID, x, y, z)
	}

	// Handle multi-block structures (doors, double plants)
	metadata := int16(blockState & 0x0F)
	isUpperHalf := metadata&0x08 != 0

	var otherY int32
	if isUpperHalf {
		otherY = y - 1
	} else {
		otherY = y + 1
	}

	// Check if the other block should also be broken
	// Doors: 64, 71, 193-197
	// Double Plants: 175
	isDoor := blockID == 64 || blockID == 71 || (blockID >= 193 && blockID <= 197)
	isDoublePlant := blockID == 175

	if isDoor || isDoublePlant {
		otherState := s.world.GetBlock(x, otherY, z)
		otherID := otherState >> 4

		// For doors, if the first part didn't give an item (e.g. upper half broken), try the other half.
		if isDoor && giveItem && itemID < 0 && otherID == blockID {
			itemID, damage, count = world.BlockToItemID(otherState)
		}

		if otherID == blockID {
			// Break the other half too
			s.world.SetBlock(x, otherY, z, 0)
			s.broadcastBlockChange(x, otherY, z, 0)
		}
	}

	// Set block to air in world state
	s.world.SetBlock(x, y, z, 0)

	// Broadcast block change (air) to all players
	s.broadcastBlockChange(x, y, z, 0)

	// In creative mode, don't give items on break
	if !giveItem {
		return
	}

	// Give item to player by spawning it on the ground
	if itemID < 0 {
		return
	}

	// Spawn item at the center of the broken block with random velocity
	vx := (rand.Float64()*0.2 - 0.1)
	vy := 0.2
	vz := (rand.Float64()*0.2 - 0.1)
	s.SpawnItem(float64(x)+0.5, float64(y)+0.5, float64(z)+0.5, vx, vy, vz, itemID, damage, count)

	log.Printf("Player %s broke block %d at (%d, %d, %d), spawned item %d:%d (count: %d)", player.Username, blockID, x, y, z, itemID, damage, count)
}

// handleBlockPlacement processes a Block Placement packet (0x08).
func (s *Server) handleBlockPlacement(player *Player, r *bytes.Reader) {
	x, y, z, _ := protocol.ReadPosition(r)
	face, _ := protocol.ReadByte(r)
	// Read held item slot data
	itemID, _, damage, _ := protocol.ReadSlotData(r)
	// Cursor position (3 bytes)
	cursorX, _ := protocol.ReadByte(r)
	cursorY, _ := protocol.ReadByte(r)
	_, _ = protocol.ReadByte(r) // cursorZ - unused

	if player.GameMode == GameModeSpectator || player.GameMode == GameModeAdventure {
		player.mu.Lock()
		slotIndex := 36 + player.ActiveSlot
		slot := player.Inventory[slotIndex]
		pkt := protocol.MarshalPacket(0x2F, func(w *bytes.Buffer) {
			protocol.WriteByte(w, 0)
			protocol.WriteInt16(w, int16(slotIndex))
			protocol.WriteSlotData(w, slot.ItemID, slot.Count, slot.Damage)
		})
		if player.Conn != nil {
			protocol.WritePacket(player.Conn, pkt)
		}
		player.mu.Unlock()
		return // spectators and adventure mode can't place blocks
	}

	// Special position (-1, -1, -1) means "use item" not placement
	if x == -1 && y == 255 && z == -1 {
		player.mu.Lock()
		slotIndex := 36 + player.ActiveSlot
		slot := player.Inventory[slotIndex]

		// Spawn eggs used in air (use item) — spawn at player's look position
		if itemID == 383 {
			px, py, pz := player.X, player.Y, player.Z
			mobType := byte(slot.Damage)
			gameMode := player.GameMode
			player.mu.Unlock()

			s.SpawnMob(px, py+1.0, pz, mobType)

			if gameMode == GameModeSurvival {
				player.mu.Lock()
				si := 36 + player.ActiveSlot
				player.Inventory[si].Count--
				if player.Inventory[si].Count <= 0 {
					player.Inventory[si] = Slot{ItemID: -1}
				}
				sl := player.Inventory[si]
				syncPkt := protocol.MarshalPacket(0x2F, func(w *bytes.Buffer) {
					protocol.WriteByte(w, 0)
					protocol.WriteInt16(w, int16(si))
					protocol.WriteSlotData(w, sl.ItemID, sl.Count, sl.Damage)
				})
				if player.Conn != nil {
					protocol.WritePacket(player.Conn, syncPkt)
				}
				player.mu.Unlock()
			}
			log.Printf("Player %s used spawn egg (mob type %d) in air", player.Username, mobType)
			return
		}

		log.Printf("Aborting USE ITEM for %d. Server thinks active slot %d (index %d) has item %d:%d qty %d", itemID, player.ActiveSlot, slotIndex, slot.ItemID, slot.Damage, slot.Count)
		pkt := protocol.MarshalPacket(0x2F, func(w *bytes.Buffer) {
			protocol.WriteByte(w, 0) // Window ID 0 = player inventory
			protocol.WriteInt16(w, int16(slotIndex))
			protocol.WriteSlotData(w, slot.ItemID, slot.Count, slot.Damage)
		})
		if player.Conn != nil {
			protocol.WritePacket(player.Conn, pkt)
		}
		player.mu.Unlock()
		return
	}

	// Check if right-clicked block is a crafting table
	clickedBlockState := s.world.GetBlock(x, y, z)
	clickedBlockID := clickedBlockState >> 4
	if clickedBlockID == 58 { // Crafting Table
		player.mu.Lock()
		player.OpenWindowID = 1
		for i := range player.CraftTableGrid {
			player.CraftTableGrid[i] = Slot{ItemID: -1}
		}
		player.CraftTableOutput = Slot{ItemID: -1}
		openPkt := protocol.MarshalPacket(0x2D, func(w *bytes.Buffer) {
			protocol.WriteByte(w, 1)                            // Window ID
			protocol.WriteString(w, "minecraft:crafting_table") // Window Type
			protocol.WriteString(w, `{"text":"Crafting"}`)      // Window Title
			protocol.WriteByte(w, 0)                            // Number of Slots
		})
		if player.Conn != nil {
			protocol.WritePacket(player.Conn, openPkt)
		}
		player.mu.Unlock()
		return
	}

	// Handle door right-click interaction (open/close)
	// Doors: 64, 71, 193-197
	if clickedBlockID == 64 || clickedBlockID == 71 || (clickedBlockID >= 193 && clickedBlockID <= 197) {
		metadata := int16(clickedBlockState & 0x0F)
		var otherY int32
		var otherState uint16
		var upperMetadata int16
		var lowerMetadata int16

		if metadata&0x08 != 0 {
			// Upper half clicked
			otherY = y - 1
			otherState = s.world.GetBlock(x, otherY, z)
			lowerMetadata = int16(otherState & 0x0F)
			upperMetadata = metadata
		} else {
			// Lower half clicked
			otherY = y + 1
			otherState = s.world.GetBlock(x, otherY, z)
			upperMetadata = int16(otherState & 0x0F)
			lowerMetadata = metadata
		}

		// Toggle the open bit (0x04) on the lower half
		newLowerMetadata := lowerMetadata ^ 0x04
		newLowerState := (clickedBlockID << 4) | uint16(newLowerMetadata)
		newUpperState := (clickedBlockID << 4) | uint16(upperMetadata) // Upper half metadata remains same

		if metadata&0x08 != 0 {
			// Clicked upper half
			s.world.SetBlock(x, y, z, newUpperState)
			s.broadcastBlockChange(x, y, z, newUpperState)
			s.world.SetBlock(x, otherY, z, newLowerState)
			s.broadcastBlockChange(x, otherY, z, newLowerState)
		} else {
			// Clicked lower half
			s.world.SetBlock(x, y, z, newLowerState)
			s.broadcastBlockChange(x, y, z, newLowerState)
			s.world.SetBlock(x, otherY, z, newUpperState)
			s.broadcastBlockChange(x, otherY, z, newUpperState)
		}

		// Send sound effect for door
		soundPkt := protocol.MarshalPacket(0x28, func(w *bytes.Buffer) {
			protocol.WriteInt32(w, 1003) // Effect ID: open/close door
			protocol.WritePosition(w, x, y, z)
			protocol.WriteInt32(w, 0)
			protocol.WriteBool(w, false)
		})
		s.mu.RLock()
		for _, p := range s.players {
			if p.EntityID == player.EntityID {
				continue
			}
			p.mu.Lock()
			if p.Conn != nil {
				protocol.WritePacket(p.Conn, soundPkt)
			}
			p.mu.Unlock()
		}
		s.mu.RUnlock()

		return // Don't place a block!
	}

	// Handle bonemeal on crops and saplings
	if itemID == 351 && damage == 15 {
		isCrop := clickedBlockID == 59 || clickedBlockID == 141 || clickedBlockID == 142 || clickedBlockID == 104 || clickedBlockID == 105
		isSapling := clickedBlockID == 6

		success := false

		if isCrop {
			currentMeta := clickedBlockState & 0x0F
			if currentMeta < 7 {
				growth := uint16(2 + rand.Intn(4))
				newMeta := currentMeta + growth
				if newMeta > 7 {
					newMeta = 7
				}
				newBlockState := (uint16(clickedBlockID) << 4) | newMeta
				s.world.SetBlock(x, y, z, newBlockState)
				s.broadcastBlockChange(x, y, z, newBlockState)
				success = true
				log.Printf("Player %s bonemealed crop %d at (%d, %d, %d) to stage %d", player.Username, clickedBlockID, x, y, z, newMeta)
			}
		} else if isSapling {
			meta := clickedBlockState & 0x0F
			woodType := meta & 0x07

			// Determine growth size
			size := 1
			tx, tz := x, z

			if woodType == 5 { // Dark Oak requires 3x3 for giant
				if rx, rz, found := s.check3x3Sapling(x, y, z, meta); found {
					size = 3
					tx, tz = rx, rz
				} else {
					// Dark Oak doesn't grow in 1x1 or 2x2
					return // Don't consume bonemeal
				}
			} else {
				if rx, rz, found := s.check2x2Sapling(x, y, z, meta); found {
					size = 2
					tx, tz = rx, rz
				}
			}

			// Grow tree (45% chance to succeed instantly per bonemeal as a simple approx)
			if rand.Float32() < 0.45 {
				s.growTree(tx, y, tz, meta, size)
			}
			success = true // Consume bonemeal even if it didn't grow this tick
		}

		if success {
			if player.GameMode == GameModeSurvival {
				player.mu.Lock()
				slotIndex := 36 + player.ActiveSlot
				if player.Inventory[slotIndex].Count > 0 {
					player.Inventory[slotIndex].Count--
					if player.Inventory[slotIndex].Count <= 0 {
						player.Inventory[slotIndex] = Slot{ItemID: -1}
					}
					slot := player.Inventory[slotIndex]
					pkt := protocol.MarshalPacket(0x2F, func(w *bytes.Buffer) {
						protocol.WriteByte(w, 0)
						protocol.WriteInt16(w, int16(slotIndex))
						protocol.WriteSlotData(w, slot.ItemID, slot.Count, slot.Damage)
					})
					if player.Conn != nil {
						protocol.WritePacket(player.Conn, pkt)
					}
				}
				player.mu.Unlock()
			}
			return
		}
	}

	// Handle spawn egg right-click on a block
	if itemID == 383 {
		tx, ty, tz := faceOffset(x, y, z, face)
		player.mu.Lock()
		slotIndex := 36 + player.ActiveSlot
		slot := player.Inventory[slotIndex]
		mobType := byte(slot.Damage)
		gameMode := player.GameMode
		player.mu.Unlock()

		s.SpawnMob(float64(tx)+0.5, float64(ty), float64(tz)+0.5, mobType)

		if gameMode == GameModeSurvival {
			player.mu.Lock()
			player.Inventory[slotIndex].Count--
			if player.Inventory[slotIndex].Count <= 0 {
				player.Inventory[slotIndex] = Slot{ItemID: -1}
			}
			sl := player.Inventory[slotIndex]
			syncPkt := protocol.MarshalPacket(0x2F, func(w *bytes.Buffer) {
				protocol.WriteByte(w, 0)
				protocol.WriteInt16(w, int16(slotIndex))
				protocol.WriteSlotData(w, sl.ItemID, sl.Count, sl.Damage)
			})
			if player.Conn != nil {
				protocol.WritePacket(player.Conn, syncPkt)
			}
			player.mu.Unlock()
		}
		log.Printf("Player %s used spawn egg (mob type %d) at (%d, %d, %d)", player.Username, mobType, tx, ty, tz)
		return
	}

	// Determine if the item is a door and map to its block ID
	var isDoor bool
	var placedBlockID int16 = itemID

	switch itemID {
	case 295: // Wheat Seeds -> Wheat Crop block
		placedBlockID = 59
	case 391: // Carrot -> Carrot Crop block
		placedBlockID = 141
	case 392: // Potato -> Potato Crop block
		placedBlockID = 142
	case 338: // Sugar Cane -> Sugar Cane block
		placedBlockID = 83
	case 361: // Pumpkin Seeds -> Pumpkin Stem block
		placedBlockID = 104
	case 362: // Melon Seeds -> Melon Stem block
		placedBlockID = 105
	case 324:
		placedBlockID = 64
		isDoor = true
	case 330:
		placedBlockID = 71
		isDoor = true
	case 427:
		placedBlockID = 193
		isDoor = true
	case 428:
		placedBlockID = 194
		isDoor = true
	case 429:
		placedBlockID = 195
		isDoor = true
	case 430:
		placedBlockID = 196
		isDoor = true
	case 431:
		placedBlockID = 197
		isDoor = true
	}

	// Don't place air
	if placedBlockID <= 0 || placedBlockID > 255 {
		player.mu.Lock()
		slotIndex := 36 + player.ActiveSlot
		slot := player.Inventory[slotIndex]
		log.Printf("Aborting place for item %d (mapped to %d). Server thinks active slot %d (index %d) has item %d:%d qty %d", itemID, placedBlockID, player.ActiveSlot, slotIndex, slot.ItemID, slot.Damage, slot.Count)
		pkt := protocol.MarshalPacket(0x2F, func(w *bytes.Buffer) {
			protocol.WriteByte(w, 0) // Window ID 0 = player inventory
			protocol.WriteInt16(w, int16(slotIndex))
			protocol.WriteSlotData(w, slot.ItemID, slot.Count, slot.Damage)
		})
		if player.Conn != nil {
			protocol.WritePacket(player.Conn, pkt)
		}
		player.mu.Unlock()
		return
	}

	// Calculate target position from face
	tx, ty, tz := faceOffset(x, y, z, face)

	// Don't place in invalid y limits
	if ty < 0 || ty > 255 {
		player.mu.Lock()
		slotIndex := 36 + player.ActiveSlot
		slot := player.Inventory[slotIndex]
		pkt := protocol.MarshalPacket(0x2F, func(w *bytes.Buffer) {
			protocol.WriteByte(w, 0)
			protocol.WriteInt16(w, int16(slotIndex))
			protocol.WriteSlotData(w, slot.ItemID, slot.Count, slot.Damage)
		})
		if player.Conn != nil {
			protocol.WritePacket(player.Conn, pkt)
		}
		player.mu.Unlock()
		return
	}

	// Don't place a block inside another non-replaceable block
	existingBlock := s.world.GetBlock(tx, ty, tz)
	existingID := existingBlock >> 4
	if existingID != 0 && existingID != 8 && existingID != 9 && existingID != 10 && existingID != 11 { // not air or liquid
		player.mu.Lock()
		slotIndex := 36 + player.ActiveSlot
		slot := player.Inventory[slotIndex]
		pkt := protocol.MarshalPacket(0x2F, func(w *bytes.Buffer) {
			protocol.WriteByte(w, 0)
			protocol.WriteInt16(w, int16(slotIndex))
			protocol.WriteSlotData(w, slot.ItemID, slot.Count, slot.Damage)
		})
		if player.Conn != nil {
			protocol.WritePacket(player.Conn, pkt)
		}
		player.mu.Unlock()
		return
	}

	// Placement validation
	validPlacement := true
	belowID := s.world.GetBlock(tx, ty-1, tz) >> 4

	if isDoor {
		// Check if we can place the top half
		topBlockID := s.world.GetBlock(tx, ty+1, tz) >> 4
		if ty >= 254 || (topBlockID != 0 && topBlockID != 8 && topBlockID != 9 && topBlockID != 10 && topBlockID != 11) {
			validPlacement = false
		}
	} else if placedBlockID == 59 || placedBlockID == 141 || placedBlockID == 142 || placedBlockID == 104 || placedBlockID == 105 { // Crops
		if belowID != 60 {
			validPlacement = false
		}
	} else if placedBlockID == 83 { // Sugar Cane
		if belowID != 12 && belowID != 3 && belowID != 2 {
			validPlacement = false
		} else {
			// Check adjacent blocks for water (8 or 9)
			waterFound := false
			adj := []struct{ dx, dz int32 }{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
			for _, d := range adj {
				adjID := s.world.GetBlock(tx+d.dx, ty-1, tz+d.dz) >> 4
				if adjID == 8 || adjID == 9 {
					waterFound = true
					break
				}
			}
			if !waterFound {
				validPlacement = false
			}
		}
	} else if placedBlockID == 81 { // Cactus
		if belowID != 12 {
			validPlacement = false
		} else {
			// Cactus cannot be placed adjacent to a solid block
			adj := []struct{ dx, dz int32 }{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
			for _, d := range adj {
				adjBlock := s.world.GetBlock(tx+d.dx, ty, tz+d.dz) >> 4
				if adjBlock != 0 && adjBlock != 8 && adjBlock != 9 && adjBlock != 10 && adjBlock != 11 {
					validPlacement = false
					break
				}
			}
		}
	} else if placedBlockID == 6 { // Sapling
		if belowID != 3 && belowID != 2 {
			validPlacement = false
		}
	}

	if !validPlacement {
		player.mu.Lock()
		slotIndex := 36 + player.ActiveSlot
		slot := player.Inventory[slotIndex]
		pkt := protocol.MarshalPacket(0x2F, func(w *bytes.Buffer) {
			protocol.WriteByte(w, 0)
			protocol.WriteInt16(w, int16(slotIndex))
			protocol.WriteSlotData(w, slot.ItemID, slot.Count, slot.Damage)
		})
		if player.Conn != nil {
			protocol.WritePacket(player.Conn, pkt)
		}
		player.mu.Unlock()
		return
	}

	// Compute correct metadata for directional blocks
	player.mu.Lock()
	yaw := player.Yaw
	player.mu.Unlock()
	metadata := blockPlacementMeta(placedBlockID, byte(damage), face, cursorX, cursorY, yaw)

	// Set block in world
	blockState := uint16(placedBlockID)<<4 | uint16(metadata)
	s.world.SetBlock(tx, ty, tz, blockState)

	// Broadcast block change to all players
	s.broadcastBlockChange(tx, ty, tz, blockState)

	if isDoor {
		// upper half has bit 0x08 set
		topBlockState := uint16(placedBlockID)<<4 | uint16(8)
		s.world.SetBlock(tx, ty+1, tz, topBlockState)
		s.broadcastBlockChange(tx, ty+1, tz, topBlockState)
	}

	// Decrement the item stack if survival
	if player.GameMode == GameModeSurvival {
		player.mu.Lock()
		slotIndex := 36 + player.ActiveSlot
		if player.Inventory[slotIndex].ItemID == itemID && player.Inventory[slotIndex].Count > 0 {
			player.Inventory[slotIndex].Count--
			if player.Inventory[slotIndex].Count <= 0 {
				player.Inventory[slotIndex] = Slot{ItemID: -1}
			}
			// Sync the slot to the client to ensure consistency
			slot := player.Inventory[slotIndex]
			pkt := protocol.MarshalPacket(0x2F, func(w *bytes.Buffer) {
				protocol.WriteByte(w, 0) // Window ID 0 = player inventory
				protocol.WriteInt16(w, int16(slotIndex))
				protocol.WriteSlotData(w, slot.ItemID, slot.Count, slot.Damage)
			})
			if player.Conn != nil {
				protocol.WritePacket(player.Conn, pkt)
			}
		}
		player.mu.Unlock()
	}

	log.Printf("Player %s placed block %d (from item %d) at (%d, %d, %d)", player.Username, placedBlockID, itemID, tx, ty, tz)
}

// check2x2Sapling checks if the sapling at (x, y, z) is part of a 2x2 square
// northwest (minimum X, minimum Z) sapling in the square, and true if found.
func (s *Server) check2x2Sapling(x, y, z int32, meta uint16) (int32, int32, bool) {
	saplingID := uint16(6)
	target := (saplingID << 4) | (meta & 0x07)

	for ox := int32(-1); ox <= 0; ox++ {
		for oz := int32(-1); oz <= 0; oz++ {
			match := true
			for dx := int32(0); dx <= 1; dx++ {
				for dz := int32(0); dz <= 1; dz++ {
					if s.world.GetBlock(x+ox+dx, y, z+oz+dz) != target {
						match = false
						break
					}
				}
				if !match {
					break
				}
			}
			if match {
				return x + ox, z + oz, true
			}
		}
	}
	return 0, 0, false
}

// check3x3Sapling checks if the sapling at (x, y, z) is part of a 3x3 square
// northwest (minimum X, minimum Z) sapling in the square, and true if found.
func (s *Server) check3x3Sapling(x, y, z int32, meta uint16) (int32, int32, bool) {
	saplingID := uint16(6)
	target := (saplingID << 4) | (meta & 0x07)

	for ox := int32(-2); ox <= 0; ox++ {
		for oz := int32(-2); oz <= 0; oz++ {
			// Check if (x+ox, y, z+oz) is the top-left of a 3x3
			match := true
			for dx := int32(0); dx <= 2; dx++ {
				for dz := int32(0); dz <= 2; dz++ {
					if s.world.GetBlock(x+ox+dx, y, z+oz+dz) != target {
						match = false
						break
					}
				}
				if !match {
					break
				}
			}
			if match {
				return x + ox, z + oz, true
			}
		}
	}
	return 0, 0, false
}

func (s *Server) growTree(x, y, z int32, saplingMeta uint16, size int) {
	// Determine log/leaf types from sapling metadata
	var logID uint16 = 17
	var leafID uint16 = 18
	// 0=oak, 1=spruce, 2=birch, 3=jungle, 4=acacia, 5=dark oak
	woodType := saplingMeta & 0x07
	if woodType > 3 {
		logID = 162
		leafID = 161
		woodType -= 4
	}

	logState := (logID << 4) | woodType
	leafState := (leafID << 4) | woodType

	// Clear sapling(s)
	if size == 3 {
		for dx := int32(0); dx <= 2; dx++ {
			for dz := int32(0); dz <= 2; dz++ {
				s.world.SetBlock(x+dx, y, z+dz, 0)
				s.broadcastBlockChange(x+dx, y, z+dz, 0)
			}
		}
	} else if size == 2 {
		for dx := int32(0); dx <= 1; dx++ {
			for dz := int32(0); dz <= 1; dz++ {
				s.world.SetBlock(x+dx, y, z+dz, 0)
				s.broadcastBlockChange(x+dx, y, z+dz, 0)
			}
		}
	} else {
		s.world.SetBlock(x, y, z, 0)
		s.broadcastBlockChange(x, y, z, 0)
	}

	switch saplingMeta & 0x07 {
	case 1: // Spruce
		if size >= 2 {
			height := int32(20 + rand.Intn(10))
			// 2x2 Trunk
			for i := int32(0); i < height; i++ {
				for dx := int32(0); dx <= 1; dx++ {
					for dz := int32(0); dz <= 1; dz++ {
						s.world.SetBlock(x+dx, y+i, z+dz, logState)
						s.broadcastBlockChange(x+dx, y+i, z+dz, logState)
					}
				}
			}
			// Organic Tiers
			for ly := y + 4; ly <= y+height+2; ly++ {
				radius := (y + height + 2 - ly) / 3
				if ly >= y+height-4 {
					radius = 1
				}
				if rand.Intn(4) == 0 {
					radius++
				} // Jitter
				for lx := x - radius; lx <= x+radius+1; lx++ {
					for lz := z - radius; lz <= z+radius+1; lz++ {
						dist := (lx-x)*(lx-x) + (lz-z)*(lz-z)
						if dist < (radius+1)*(radius+1)-int32(rand.Intn(2)) {
							if s.world.GetBlock(lx, ly, lz)>>4 == 0 {
								s.world.SetBlock(lx, ly, lz, leafState)
								s.broadcastBlockChange(lx, ly, lz, leafState)
							}
						}
					}
				}
			}
		} else {
			height := int32(7 + rand.Intn(3))
			for i := int32(0); i < height; i++ {
				s.world.SetBlock(x, y+i, z, logState)
				s.broadcastBlockChange(x, y+i, z, logState)
			}
			for ly := y + 2; ly <= y+height; ly++ {
				radius := (y + height - ly) / 2
				if ly == y+height {
					radius = 0
				} else if (y+height-ly)%2 == 0 {
					radius++
				}
				for lx := x - radius; lx <= x+radius; lx++ {
					for lz := z - radius; lz <= z+radius; lz++ {
						if (lx-x)*(lx-x)+(lz-z)*(lz-z) <= radius*radius {
							if s.world.GetBlock(lx, ly, lz)>>4 == 0 {
								s.world.SetBlock(lx, ly, lz, leafState)
								s.broadcastBlockChange(lx, ly, lz, leafState)
							}
						}
					}
				}
			}
		}
	case 2: // Birch
		if size >= 2 {
			height := int32(15 + rand.Intn(8))
			// 2x2 Trunk
			for i := int32(0); i < height; i++ {
				for dx := int32(0); dx <= 1; dx++ {
					for dz := int32(0); dz <= 1; dz++ {
						s.world.SetBlock(x+dx, y+i, z+dz, logState)
						s.broadcastBlockChange(x+dx, y+i, z+dz, logState)
					}
				}
			}
			// Organic canopy with small limbs
			for _, dr := range []struct{ dx, dz int32 }{{1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
				lx, ly, lz := x, y+height-4, z
				if dr.dx > 0 {
					lx++
				}
				if dr.dz > 0 {
					lz++
				}
				for i := 0; i < 4; i++ {
					ly++
					lx += dr.dx
					lz += dr.dz
					s.world.SetBlock(lx, ly, lz, logState)
					s.broadcastBlockChange(lx, ly, lz, logState)
					r := int32(2 + rand.Intn(2))
					for lly := ly; lly <= ly+2; lly++ {
						for llx := lx - r; llx <= lx+r; llx++ {
							for llz := lz - r; llz <= lz+r; llz++ {
								if (llx-lx)*(llx-lx)+(llz-lz)*(llz-lz) < r*r {
									if s.world.GetBlock(llx, lly, llz)>>4 == 0 {
										s.world.SetBlock(llx, lly, llz, leafState)
										s.broadcastBlockChange(llx, lly, llz, leafState)
									}
								}
							}
						}
					}
				}
			}
		} else {
			height := int32(5 + rand.Intn(3))
			for i := int32(0); i < height; i++ {
				s.world.SetBlock(x, y+i, z, logState)
				s.broadcastBlockChange(x, y+i, z, logState)
			}
			for ly := y + height - 3; ly <= y+height+1; ly++ {
				radius := int32(2)
				if ly >= y+height {
					radius = 1
				}
				for lx := x - radius; lx <= x+radius; lx++ {
					for lz := z - radius; lz <= z+radius; lz++ {
						if (lx-x)*(lx-x)+(lz-z)*(lz-z) <= radius*radius+1 {
							if s.world.GetBlock(lx, ly, lz)>>4 == 0 {
								s.world.SetBlock(lx, ly, lz, leafState)
								s.broadcastBlockChange(lx, ly, lz, leafState)
							}
						}
					}
				}
			}
		}

	case 3: // Jungle
		if size >= 2 {
			height := int32(25 + rand.Intn(15))
			for i := int32(0); i < height; i++ {
				for dx := int32(0); dx <= 1; dx++ {
					for dz := int32(0); dz <= 1; dz++ {
						s.world.SetBlock(x+dx, y+i, z+dz, logState)
						s.broadcastBlockChange(x+dx, y+i, z+dz, logState)
					}
				}
				// Buttress roots at base
				if i < 6 {
					off := []struct{ dx, dz int32 }{{-1, 0}, {2, 0}, {0, -1}, {0, 2}, {1, -1}, {1, 2}, {-1, 1}, {2, 1}}
					for _, o := range off {
						if i < 5-int32(math.Abs(float64(o.dx)))-int32(math.Abs(float64(o.dz))) {
							s.world.SetBlock(x+o.dx, y+i, z+o.dz, logState)
							s.broadcastBlockChange(x+o.dx, y+i, z+o.dz, logState)
						}
					}
				}
			}
			// Clumped foliage
			for h := int32(10); h < height+3; h += 4 {
				r := int32(3 + rand.Intn(3))
				for ly := y + h; ly < y+h+3; ly++ {
					for lx := x - r; lx <= x+r+1; lx++ {
						for lz := z - r; lz <= z+r+1; lz++ {
							if (lx-x)*(lx-x)+(lz-z)*(lz-z) < r*r+int32(rand.Intn(3)) {
								if s.world.GetBlock(lx, ly, lz)>>4 == 0 {
									s.world.SetBlock(lx, ly, lz, leafState)
									s.broadcastBlockChange(lx, ly, lz, leafState)
								}
							}
						}
					}
				}
			}
		} else {
			height := int32(10 + rand.Intn(4))
			for i := int32(0); i < height; i++ {
				s.world.SetBlock(x, y+i, z, logState)
				s.broadcastBlockChange(x, y+i, z, logState)
			}
			for ly := y + height - 2; ly <= y+height+1; ly++ {
				radius := int32(2)
				if ly > y+height-1 {
					radius = 1
				}
				for lx := x - radius; lx <= x+radius; lx++ {
					for lz := z - radius; lz <= z+radius; lz++ {
						if (lx-x)*(lx-x)+(lz-z)*(lz-z) <= radius*radius+1 {
							if s.world.GetBlock(lx, ly, lz)>>4 == 0 {
								s.world.SetBlock(lx, ly, lz, leafState)
								s.broadcastBlockChange(lx, ly, lz, leafState)
							}
						}
					}
				}
			}
		}

	case 4: // Acacia
		if size >= 2 {
			height := int32(10 + rand.Intn(6))
			for i := int32(0); i < height; i++ {
				for dx := int32(0); dx <= 1; dx++ {
					for dz := int32(0); dz <= 1; dz++ {
						s.world.SetBlock(x+dx, y+i, z+dz, logState)
						s.broadcastBlockChange(x+dx, y+i, z+dz, logState)
					}
				}
			}
			for _, dir := range []struct{ dx, dz int32 }{{1, 1}, {-1, 1}, {1, -1}, {-1, -1}, {2, 0}, {-2, 0}, {0, 2}, {0, -2}} {
				by := y + height/2 + int32(rand.Intn(int(height/2)))
				bx, bz := x, z
				if dir.dx > 0 {
					bx++
				}
				if dir.dz > 0 {
					bz++
				}
				blen := 4 + rand.Intn(4)
				for i := 0; i < blen; i++ {
					by += int32(i / 3)
					bx += dir.dx
					bz += dir.dz
					s.world.SetBlock(bx, by, bz, logState)
					s.broadcastBlockChange(bx, by, bz, logState)
					if i >= blen-2 {
						for ly := by; ly <= by+1; ly++ {
							r := int32(2)
							for lx := bx - r; lx <= bx+r; lx++ {
								for lz := bz - r; lz <= bz+r; lz++ {
									if s.world.GetBlock(lx, ly, lz)>>4 == 0 {
										s.world.SetBlock(lx, ly, lz, leafState)
										s.broadcastBlockChange(lx, ly, lz, leafState)
									}
								}
							}
						}
					}
				}
			}
		} else {
			height := int32(5 + rand.Intn(3))
			for i := int32(0); i < height; i++ {
				s.world.SetBlock(x, y+i, z, logState)
				s.broadcastBlockChange(x, y+i, z, logState)
			}
			dx, dz := int32(1), int32(1)
			if rand.Intn(2) == 0 {
				dx = -1
			}
			if rand.Intn(2) == 0 {
				dz = -1
			}
			bx, by, bz := x, y+height-1, z
			for i := 0; i < 3; i++ {
				by++
				if i > 0 {
					bx += dx
					bz += dz
				}
				s.world.SetBlock(bx, by, bz, logState)
				s.broadcastBlockChange(bx, by, bz, logState)
			}
			for ly := by; ly <= by+1; ly++ {
				radius := int32(2)
				if ly > by {
					radius = 1
				}
				for lx := bx - radius; lx <= bx+radius; lx++ {
					for lz := bz - radius; lz <= bz+radius; lz++ {
						if s.world.GetBlock(lx, ly, lz)>>4 == 0 {
							s.world.SetBlock(lx, ly, lz, leafState)
							s.broadcastBlockChange(lx, ly, lz, leafState)
						}
					}
				}
			}
		}

	case 5: // Dark Oak (Size 3 Giant)
		if size >= 3 {
			height := int32(15 + rand.Intn(10))
			// 3x3 Trunk
			for i := int32(0); i < height; i++ {
				for dx := int32(0); dx <= 2; dx++ {
					for dz := int32(0); dz <= 2; dz++ {
						s.world.SetBlock(x+dx, y+i, z+dz, logState)
						s.broadcastBlockChange(x+dx, y+i, z+dz, logState)
					}
				}
			}
			// Organic Roots & Limbs
			dirs := []struct{ dx, dz int32 }{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, -1}, {1, -1}, {-1, 1}}
			for _, d := range dirs {
				// Base roots
				for i := int32(1); i <= 3; i++ {
					s.world.SetBlock(x+1+d.dx*i, y, z+1+d.dz*i, logState)
					s.broadcastBlockChange(x+1+d.dx*i, y, z+1+d.dz*i, logState)
				}
				// Branch limbs
				lx, ly, lz := x+1, y+height/3+int32(rand.Intn(int(height/2))), z+1
				blen := 6 + rand.Intn(6)
				for i := 0; i < blen; i++ {
					ly += int32(i / 3)
					lx += d.dx
					lz += d.dz
					s.world.SetBlock(lx, ly, lz, logState)
					s.broadcastBlockChange(lx, ly, lz, logState)
					if i >= blen-4 {
						r := int32(3 + rand.Intn(2))
						for lly := ly - 1; lly <= ly+2; lly++ {
							for llx := lx - r; llx <= lx+r; llx++ {
								for llz := lz - r; llz <= lz+r; llz++ {
									if (llx-lx)*(llx-lx)+(llz-lz)*(llz-lz) < r*r {
										if s.world.GetBlock(llx, lly, llz)>>4 == 0 {
											s.world.SetBlock(llx, lly, llz, leafState)
											s.broadcastBlockChange(llx, lly, llz, leafState)
										}
									}
								}
							}
						}
					}
				}
			}
		} else {
			height := int32(4 + rand.Intn(2))
			for i := int32(0); i < height; i++ {
				s.world.SetBlock(x, y+i, z, logState)
				s.broadcastBlockChange(x, y+i, z, logState)
			}
		}

	default: // Oak
		if size >= 2 {
			height := int32(12 + rand.Intn(8))
			// 2x2 Trunk spreading at top
			for i := int32(0); i < height; i++ {
				for dx := int32(0); dx <= 1; dx++ {
					for dz := int32(0); dz <= 1; dz++ {
						s.world.SetBlock(x+dx, y+i, z+dz, logState)
						s.broadcastBlockChange(x+dx, y+i, z+dz, logState)
					}
				}
			}
			// Spreading organic limbs
			for _, dr := range []struct{ dx, dz int32 }{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, -1}, {1, -1}, {-1, 1}} {
				lx, ly, lz := x, y+height/2+int32(rand.Intn(int(height/2))), z
				if dr.dx > 0 {
					lx++
				}
				if dr.dz > 0 {
					lz++
				}

				limbLen := 3 + rand.Intn(5)
				for i := 0; i < limbLen; i++ {
					ly += int32(i / 2)
					lx += dr.dx
					lz += dr.dz
					s.world.SetBlock(lx, ly, lz, logState)
					s.broadcastBlockChange(lx, ly, lz, logState)

					// Leaf clumps at limb ends and along
					if i >= limbLen-3 {
						r := int32(2 + rand.Intn(2))
						for lly := ly - 1; lly <= ly+1; lly++ {
							for llx := lx - r; llx <= lx+r; llx++ {
								for llz := lz - r; llz <= lz+r; llz++ {
									if (llx-lx)*(llx-lx)+(llz-lz)*(llz-lz) < r*r {
										if s.world.GetBlock(llx, lly, llz)>>4 == 0 {
											s.world.SetBlock(llx, lly, llz, leafState)
											s.broadcastBlockChange(llx, lly, llz, leafState)
										}
									}
								}
							}
						}
					}
				}
			}
		} else {
			// Standard 1x1 Oak
			height := int32(4 + rand.Intn(3))
			for i := int32(0); i < height; i++ {
				s.world.SetBlock(x, y+i, z, logState)
				s.broadcastBlockChange(x, y+i, z, logState)
			}
			for ly := y + height - 2; ly <= y+height+1; ly++ {
				radius := int32(2)
				if ly >= y+height {
					radius = 1
				}
				for lx := x - radius; lx <= x+radius; lx++ {
					for lz := z - radius; lz <= z+radius; lz++ {
						if (lx-x)*(lx-x)+(lz-z)*(lz-z) <= radius*radius+1 {
							if s.world.GetBlock(lx, ly, lz)>>4 == 0 {
								s.world.SetBlock(lx, ly, lz, leafState)
								s.broadcastBlockChange(lx, ly, lz, leafState)
							}
						}
					}
				}
			}
		}
	}

	log.Printf("Tree grown at (%d, %d, %d) type %d", x, y, z, saplingMeta&0x07)
}

// faceOffset returns the target block position when placing against a face.
// Face values: 0=bottom, 1=top, 2=north(-Z), 3=south(+Z), 4=west(-X), 5=east(+X)
func faceOffset(x, y, z int32, face byte) (int32, int32, int32) {
	switch face {
	case 0:
		return x, y - 1, z
	case 1:
		return x, y + 1, z
	case 2:
		return x, y, z - 1
	case 3:
		return x, y, z + 1
	case 4:
		return x - 1, y, z
	case 5:
		return x + 1, y, z
	default:
		return x, y + 1, z
	}
}

// yawToDirection converts a player yaw angle to a cardinal direction index.
// Returns: 0=south, 1=west, 2=north, 3=east (matches vanilla Minecraft).
func yawToDirection(yaw float32) int {
	return int(math.Floor(float64(yaw)*4.0/360.0+0.5)) & 3
}

// blockPlacementMeta computes the block metadata for a placed block based on its
// type, the item damage value, the face clicked, cursor position, and player yaw.
// For directional blocks (stairs, torches, levers, etc.) the metadata encodes
// orientation derived from placement context. For other blocks the item damage
// value is used directly as metadata (e.g. wool colour, wood type).
func blockPlacementMeta(blockID int16, itemDamage byte, face byte, cursorX byte, cursorY byte, yaw float32) byte {
	dir := yawToDirection(yaw)

	switch blockID {
	// --- Door ---
	case 64, 71, 193, 194, 195, 196, 197:
		switch dir {
		case 0:
			return 1 // South => 1
		case 1:
			return 2 // West => 2
		case 2:
			return 3 // North => 3
		case 3:
			return 0 // East => 0
		default:
			return 0
		}

	// --- Stairs ---
	case 53, 67, 108, 109, 114, 128, 134, 135, 136, 156, 163, 164, 180:
		var meta byte
		switch dir {
		case 0:
			meta = 2 // south
		case 1:
			meta = 1 // west
		case 2:
			meta = 3 // north
		case 3:
			meta = 0 // east
		}
		if face == 0 || (face != 1 && cursorY >= 8) {
			meta |= 4
		}
		return meta

	// --- Torch / Redstone Torch ---
	case 50, 75, 76:
		switch face {
		case 1:
			return 5 // floor
		case 2:
			return 4 // attached to block south, pointing north
		case 3:
			return 3 // attached to block north, pointing south
		case 4:
			return 2 // attached to block east, pointing west
		case 5:
			return 1 // attached to block west, pointing east
		default:
			return 5
		}

	// --- Lever ---
	case 69:
		switch face {
		case 0: // ceiling
			if dir == 0 || dir == 2 {
				return 7 // N-S axis
			}
			return 0 // E-W axis
		case 1: // floor
			if dir == 0 || dir == 2 {
				return 5 // N-S axis
			}
			return 6 // E-W axis
		case 2:
			return 4
		case 3:
			return 3
		case 4:
			return 2
		case 5:
			return 1
		default:
			return 5
		}

	// --- Ladder / Wall Sign ---
	case 65, 68:
		switch face {
		case 2:
			return 2
		case 3:
			return 3
		case 4:
			return 4
		case 5:
			return 5
		default:
			return 2
		}

	// --- Button (stone / wood) ---
	case 77, 143:
		switch face {
		case 0:
			return 0 // ceiling
		case 1:
			return 5 // floor
		case 2:
			return 4
		case 3:
			return 3
		case 4:
			return 2
		case 5:
			return 1
		default:
			return 5
		}

	// --- Furnace / Dispenser / Dropper ---
	case 61, 23, 158:
		switch dir {
		case 0:
			return 2 // north
		case 1:
			return 5 // east
		case 2:
			return 3 // south
		case 3:
			return 4 // west
		default:
			return 2
		}

	// --- Chest / Trapped Chest / Ender Chest ---
	case 54, 146, 130:
		switch dir {
		case 0:
			return 2
		case 1:
			return 5
		case 2:
			return 3
		case 3:
			return 4
		default:
			return 2
		}

	// --- Pumpkin / Jack-o-Lantern ---
	case 86, 91:
		return byte((dir + 2) & 3)

	// --- Log / Log2 ---
	case 17, 162:
		woodType := itemDamage & 0x03
		switch face {
		case 2, 3:
			return woodType | 8 // Z axis
		case 4, 5:
			return woodType | 4 // X axis
		default:
			return woodType // Y axis (face 0, 1, or default)
		}

	// --- Slab / Wooden Slab ---
	case 44, 126:
		slabType := itemDamage & 0x07
		if face == 0 || (face != 1 && cursorY >= 8) {
			slabType |= 8 // upper slab
		}
		return slabType

	// --- Standing Sign ---
	case 63:
		return byte(int(math.Floor(float64(yaw+180.0)*16.0/360.0+0.5)) & 15)

	// --- Hopper ---
	case 154:
		switch face {
		case 2:
			return 2
		case 3:
			return 3
		case 4:
			return 4
		case 5:
			return 5
		default:
			return 0 // output down
		}

	// --- Anvil ---
	case 145:
		return byte(dir & 3)

	// --- Redstone Repeater / Comparator ---
	case 93, 149:
		return byte(dir)

	default:
		// Non-directional blocks: use item damage as metadata (colour, variant, etc.)
		return itemDamage & 0x0F
	}
}
