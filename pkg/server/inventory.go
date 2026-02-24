package server

import (
	"bytes"
	"log"
	"math"

	"github.com/VibeShit/VibeShitCraft/pkg/protocol"
)

// addItemToInventory finds a suitable slot and adds the item to the player's inventory.
// Returns the slot index and true if successful, or -1 and false if inventory is full.
// Must be called with player.mu held.
func addItemToInventory(player *Player, itemID int16, damage int16, count byte) (int, bool) {
	// Try to stack in hotbar (slots 36-44)
	for i := 36; i <= 44; i++ {
		if player.Inventory[i].ItemID == itemID && player.Inventory[i].Damage == damage && player.Inventory[i].Count+count <= 64 {
			player.Inventory[i].Count += count
			return i, true
		}
	}
	// Try to stack in main inventory (slots 9-35)
	for i := 9; i <= 35; i++ {
		if player.Inventory[i].ItemID == itemID && player.Inventory[i].Damage == damage && player.Inventory[i].Count+count <= 64 {
			player.Inventory[i].Count += count
			return i, true
		}
	}
	// Try empty slot in hotbar
	for i := 36; i <= 44; i++ {
		if player.Inventory[i].ItemID == -1 {
			player.Inventory[i] = Slot{ItemID: itemID, Damage: damage, Count: count}
			return i, true
		}
	}
	// Try empty slot in main inventory
	for i := 9; i <= 35; i++ {
		if player.Inventory[i].ItemID == -1 {
			player.Inventory[i] = Slot{ItemID: itemID, Damage: damage, Count: count}
			return i, true
		}
	}
	return -1, false
}

// handleCreativeInventory processes Creative Inventory Action (0x10).
func (s *Server) handleCreativeInventory(player *Player, r *bytes.Reader) {
	slotNum, _ := protocol.ReadInt16(r)
	itemID, count, damage, _ := protocol.ReadSlotData(r)

	if player.GameMode != GameModeCreative {
		return
	}

	// Validate slot range (0-44 for player inventory, -1 for dropping)
	if slotNum == -1 {
		// Player is dropping an item
		player.mu.Lock()
		px, py, pz := player.X, player.Y, player.Z

		// Drop base velocity calculations relative to player direction
		f1 := math.Sin(float64(player.Yaw) * math.Pi / 180.0)
		f2 := math.Cos(float64(player.Yaw) * math.Pi / 180.0)
		f3 := math.Sin(float64(player.Pitch) * math.Pi / 180.0)
		f4 := math.Cos(float64(player.Pitch) * math.Pi / 180.0)

		vx := -f1 * f4 * 0.3
		vy := -f3*0.3 + 0.1
		vz := f2 * f4 * 0.3

		player.mu.Unlock()
		if itemID != -1 {
			s.SpawnItem(px, py+1.5, pz, vx, vy, vz, itemID, damage, count)
			log.Printf("Player %s dropped item %d:%d (creative)", player.Username, itemID, damage)
		}
		return
	}
	if slotNum < 0 || slotNum > 44 {
		return
	}

	player.mu.Lock()
	if itemID == -1 {
		// Clearing slot
		player.Inventory[slotNum] = Slot{ItemID: -1}
	} else {
		player.Inventory[slotNum] = Slot{ItemID: itemID, Count: count, Damage: damage}
	}
	player.mu.Unlock()

	// Inventory contents have changed; if this touched the currently
	// active hotbar slot, broadcast the updated held item.
	s.broadcastHeldItem(player)
}

// handleCloseWindow processes Close Window (0x0D).
func (s *Server) handleCloseWindow(player *Player, r *bytes.Reader) {
	windowID, _ := protocol.ReadByte(r)
	player.mu.Lock()
	var dropItems []Slot
	if windowID == 0 {
		// Return items from 2x2 crafting grid to inventory
		for i := 1; i <= 4; i++ {
			if player.Inventory[i].ItemID != -1 {
				_, ok := addItemToInventory(player, player.Inventory[i].ItemID, player.Inventory[i].Damage, player.Inventory[i].Count)
				if !ok {
					dropItems = append(dropItems, player.Inventory[i])
				}
				player.Inventory[i] = Slot{ItemID: -1}
			}
		}
		player.Inventory[0] = Slot{ItemID: -1}
	} else if windowID == player.OpenWindowID {
		// Return items from crafting table grid to inventory
		for i := 0; i < 9; i++ {
			if player.CraftTableGrid[i].ItemID != -1 {
				_, ok := addItemToInventory(player, player.CraftTableGrid[i].ItemID, player.CraftTableGrid[i].Damage, player.CraftTableGrid[i].Count)
				if !ok {
					dropItems = append(dropItems, player.CraftTableGrid[i])
				}
				player.CraftTableGrid[i] = Slot{ItemID: -1}
			}
		}
		player.CraftTableOutput = Slot{ItemID: -1}
		player.OpenWindowID = 0
	}
	if player.Cursor.ItemID != -1 {
		_, ok := addItemToInventory(player, player.Cursor.ItemID, player.Cursor.Damage, player.Cursor.Count)
		if !ok {
			dropItems = append(dropItems, player.Cursor)
		}
		player.Cursor = Slot{ItemID: -1}
	}
	px, py, pz := player.X, player.Y, player.Z
	player.mu.Unlock()
	for _, item := range dropItems {
		s.SpawnItem(px, py+1.5, pz, 0, 0.2, 0, item.ItemID, item.Damage, item.Count)
	}
}

// handleInventoryClick processes Click Window (0x0E) for player inventory (window ID 0).
func (s *Server) handleInventoryClick(player *Player, r *bytes.Reader) {
	windowID, _ := protocol.ReadByte(r)
	slotNum, _ := protocol.ReadInt16(r)
	button, _ := protocol.ReadByte(r)
	actionNum, _ := protocol.ReadInt16(r)
	mode, _ := protocol.ReadByte(r)
	// Read held item slot data
	itemID, _, _, _ := protocol.ReadSlotData(r)
	_ = itemID

	// Delegate non-player-inventory windows to separate handler
	if windowID != 0 {
		s.handleWindowClick(player, windowID, slotNum, button, actionNum, mode)
		return
	}

	player.mu.Lock()
	px, py, pz := player.X, player.Y, player.Z

	// Crafting output slot (slot 0) has special handling
	if slotNum == 0 {
		if mode == 0 && player.Inventory[0].ItemID != -1 {
			// Normal click on crafting output: take the result
			result := player.Inventory[0]
			if player.Cursor.ItemID == -1 {
				player.Cursor = result
				consumeCraftIngredients2x2(player)
			} else if player.Cursor.ItemID == result.ItemID && player.Cursor.Damage == result.Damage && int(player.Cursor.Count)+int(result.Count) <= 64 {
				player.Cursor.Count += result.Count
				consumeCraftIngredients2x2(player)
			}
		} else if mode == 1 && player.Inventory[0].ItemID != -1 {
			// Shift-click: craft all possible
			for player.Inventory[0].ItemID != -1 {
				result := player.Inventory[0]
				_, ok := addItemToInventory(player, result.ItemID, result.Damage, result.Count)
				if !ok {
					break
				}
				consumeCraftIngredients2x2(player)
				updateCraftOutput2x2(player)
			}
		}
	} else if slotNum >= 1 && slotNum < 45 {
		if mode == 0 { // Normal click
			if button == 0 { // Left click
				if player.Cursor.ItemID == player.Inventory[slotNum].ItemID && player.Cursor.Damage == player.Inventory[slotNum].Damage && player.Cursor.ItemID != -1 {
					space := 64 - player.Inventory[slotNum].Count
					if player.Cursor.Count <= space {
						player.Inventory[slotNum].Count += player.Cursor.Count
						player.Cursor = Slot{ItemID: -1}
					} else {
						player.Inventory[slotNum].Count = 64
						player.Cursor.Count -= space
					}
				} else { // Swap
					temp := player.Inventory[slotNum]
					player.Inventory[slotNum] = player.Cursor
					player.Cursor = temp
				}
			} else if button == 1 { // Right click
				if player.Cursor.ItemID == -1 && player.Inventory[slotNum].ItemID != -1 {
					half := (player.Inventory[slotNum].Count + 1) / 2
					player.Cursor = player.Inventory[slotNum]
					player.Cursor.Count = half
					player.Inventory[slotNum].Count -= half
					if player.Inventory[slotNum].Count == 0 {
						player.Inventory[slotNum] = Slot{ItemID: -1}
					}
				} else if player.Cursor.ItemID != -1 && player.Inventory[slotNum].ItemID == -1 {
					player.Inventory[slotNum] = player.Cursor
					player.Inventory[slotNum].Count = 1
					player.Cursor.Count--
					if player.Cursor.Count == 0 {
						player.Cursor = Slot{ItemID: -1}
					}
				} else if player.Cursor.ItemID == player.Inventory[slotNum].ItemID && player.Cursor.Damage == player.Inventory[slotNum].Damage {
					if player.Inventory[slotNum].Count < 64 {
						player.Inventory[slotNum].Count++
						player.Cursor.Count--
						if player.Cursor.Count == 0 {
							player.Cursor = Slot{ItemID: -1}
						}
					}
				} else { // Swap
					temp := player.Inventory[slotNum]
					player.Inventory[slotNum] = player.Cursor
					player.Cursor = temp
				}
			}
		} else if mode == 1 { // Shift-click
			if player.Inventory[slotNum].ItemID != -1 {
				item := player.Inventory[slotNum]
				moved := false
				var destStart, destEnd int
				if slotNum >= 36 && slotNum <= 44 {
					destStart, destEnd = 9, 35
				} else if slotNum >= 9 && slotNum <= 35 {
					destStart, destEnd = 36, 44
				} else if slotNum >= 5 && slotNum <= 8 {
					destStart, destEnd = 36, 44
				} else {
					destStart, destEnd = 9, 35
				}
				// First pass: try to stack onto existing matching items
				remaining := item.Count
				for i := destStart; i <= destEnd && remaining > 0; i++ {
					if player.Inventory[i].ItemID == item.ItemID && player.Inventory[i].Damage == item.Damage && player.Inventory[i].Count < 64 {
						space := 64 - player.Inventory[i].Count
						if remaining <= space {
							player.Inventory[i].Count += remaining
							remaining = 0
						} else {
							player.Inventory[i].Count = 64
							remaining -= space
						}
					}
				}
				// Second pass: put remainder in empty slots
				for i := destStart; i <= destEnd && remaining > 0; i++ {
					if player.Inventory[i].ItemID == -1 {
						player.Inventory[i] = Slot{ItemID: item.ItemID, Damage: item.Damage, Count: remaining}
						remaining = 0
					}
				}
				if remaining == 0 {
					player.Inventory[slotNum] = Slot{ItemID: -1}
					moved = true
				} else if remaining < item.Count {
					player.Inventory[slotNum].Count = remaining
				}
				// For armor slots, if not moved try main inventory as fallback
				if !moved && (slotNum >= 5 && slotNum <= 8) {
					remaining = player.Inventory[slotNum].Count
					for i := 9; i <= 35 && remaining > 0; i++ {
						if player.Inventory[i].ItemID == item.ItemID && player.Inventory[i].Damage == item.Damage && player.Inventory[i].Count < 64 {
							space := 64 - player.Inventory[i].Count
							if remaining <= space {
								player.Inventory[i].Count += remaining
								remaining = 0
							} else {
								player.Inventory[i].Count = 64
								remaining -= space
							}
						}
					}
					for i := 9; i <= 35 && remaining > 0; i++ {
						if player.Inventory[i].ItemID == -1 {
							player.Inventory[i] = Slot{ItemID: item.ItemID, Damage: item.Damage, Count: remaining}
							remaining = 0
						}
					}
					if remaining == 0 {
						player.Inventory[slotNum] = Slot{ItemID: -1}
					} else if remaining < player.Inventory[slotNum].Count {
						player.Inventory[slotNum].Count = remaining
					}
				}
			}
		} else if mode == 2 { // Number key hotkey
			// button = hotkey number (0-8), maps to hotbar slot 36+button
			hotbarSlot := int16(36) + int16(button)
			if hotbarSlot >= 36 && hotbarSlot <= 44 {
				temp := player.Inventory[slotNum]
				player.Inventory[slotNum] = player.Inventory[hotbarSlot]
				player.Inventory[hotbarSlot] = temp
			}
		}
	}

	// Mode 6 is double-click to collect matching items onto cursor
	if mode == 6 && player.Cursor.ItemID != -1 {
		for i := 1; i < 45 && player.Cursor.Count < 64; i++ {
			if player.Inventory[i].ItemID == player.Cursor.ItemID && player.Inventory[i].Damage == player.Cursor.Damage {
				space := 64 - player.Cursor.Count
				if player.Inventory[i].Count <= space {
					player.Cursor.Count += player.Inventory[i].Count
					player.Inventory[i] = Slot{ItemID: -1}
				} else {
					player.Cursor.Count = 64
					player.Inventory[i].Count -= space
				}
			}
		}
	}

	// Mode 5 is drag/paint (hold click and drag across slots)
	if mode == 5 {
		switch button {
		case 0: // Left drag start
			player.DragSlots = nil
			player.DragButton = 0
		case 4: // Right drag start
			player.DragSlots = nil
			player.DragButton = 1
		case 1: // Left drag add slot
			if slotNum >= 1 && slotNum < 45 {
				player.DragSlots = append(player.DragSlots, slotNum)
			}
		case 5: // Right drag add slot
			if slotNum >= 1 && slotNum < 45 {
				player.DragSlots = append(player.DragSlots, slotNum)
			}
		case 2: // Left drag end - distribute evenly
			if player.Cursor.ItemID != -1 && len(player.DragSlots) > 0 {
				perSlot := player.Cursor.Count / byte(len(player.DragSlots))
				if perSlot < 1 {
					perSlot = 1
				}
				for _, ds := range player.DragSlots {
					if player.Cursor.Count <= 0 {
						break
					}
					if player.Inventory[ds].ItemID == -1 {
						give := perSlot
						if give > player.Cursor.Count {
							give = player.Cursor.Count
						}
						player.Inventory[ds] = Slot{ItemID: player.Cursor.ItemID, Damage: player.Cursor.Damage, Count: give}
						player.Cursor.Count -= give
					} else if player.Inventory[ds].ItemID == player.Cursor.ItemID && player.Inventory[ds].Damage == player.Cursor.Damage {
						space := 64 - player.Inventory[ds].Count
						give := perSlot
						if give > space {
							give = space
						}
						if give > player.Cursor.Count {
							give = player.Cursor.Count
						}
						player.Inventory[ds].Count += give
						player.Cursor.Count -= give
					}
				}
				if player.Cursor.Count <= 0 {
					player.Cursor = Slot{ItemID: -1}
				}
			}
			player.DragSlots = nil
		case 6: // Right drag end - place one per slot
			if player.Cursor.ItemID != -1 && len(player.DragSlots) > 0 {
				for _, ds := range player.DragSlots {
					if player.Cursor.Count <= 0 {
						break
					}
					if player.Inventory[ds].ItemID == -1 {
						player.Inventory[ds] = Slot{ItemID: player.Cursor.ItemID, Damage: player.Cursor.Damage, Count: 1}
						player.Cursor.Count--
					} else if player.Inventory[ds].ItemID == player.Cursor.ItemID && player.Inventory[ds].Damage == player.Cursor.Damage && player.Inventory[ds].Count < 64 {
						player.Inventory[ds].Count++
						player.Cursor.Count--
					}
				}
				if player.Cursor.Count <= 0 {
					player.Cursor = Slot{ItemID: -1}
				}
			}
			player.DragSlots = nil
		}
	}

	// Mode 4 is drop from window
	if mode == 4 && player.GameMode != GameModeSpectator {
		if slotNum == -999 { // Drop from cursor
			if player.Cursor.ItemID != -1 {
				// Save item data BEFORE modifying cursor
				vitemID := player.Cursor.ItemID
				vdamage := player.Cursor.Damage
				dropCount := player.Cursor.Count
				if button == 0 { // Left click drops 1
					dropCount = 1
					player.Cursor.Count--
					if player.Cursor.Count <= 0 {
						player.Cursor = Slot{ItemID: -1}
					}
				} else {
					player.Cursor = Slot{ItemID: -1}
				}

				f1 := math.Sin(float64(player.Yaw) * math.Pi / 180.0)
				f2 := math.Cos(float64(player.Yaw) * math.Pi / 180.0)
				f3 := math.Sin(float64(player.Pitch) * math.Pi / 180.0)
				f4 := math.Cos(float64(player.Pitch) * math.Pi / 180.0)

				vx := -f1 * f4 * 0.3
				vy := -f3*0.3 + 0.1
				vz := f2 * f4 * 0.3

				player.mu.Unlock() // unlock to spawn
				s.SpawnItem(px, py+1.5, pz, vx, vy, vz, vitemID, vdamage, dropCount)
				player.mu.Lock()
			}
		} else if slotNum >= 1 && slotNum < 45 {
			if player.Inventory[slotNum].ItemID != -1 {
				// Save item data BEFORE modifying slot
				dropItemID := player.Inventory[slotNum].ItemID
				dropDamage := player.Inventory[slotNum].Damage
				dropCount := player.Inventory[slotNum].Count
				if button == 0 { // Q drops 1
					dropCount = 1
					player.Inventory[slotNum].Count--
					if player.Inventory[slotNum].Count <= 0 {
						player.Inventory[slotNum] = Slot{ItemID: -1}
					}
				} else { // Ctrl+Q drops all
					player.Inventory[slotNum] = Slot{ItemID: -1}
				}

				f1 := math.Sin(float64(player.Yaw) * math.Pi / 180.0)
				f2 := math.Cos(float64(player.Yaw) * math.Pi / 180.0)
				f3 := math.Sin(float64(player.Pitch) * math.Pi / 180.0)
				f4 := math.Cos(float64(player.Pitch) * math.Pi / 180.0)

				vx := -f1 * f4 * 0.3
				vy := -f3*0.3 + 0.1
				vz := f2 * f4 * 0.3

				player.mu.Unlock() // unlock to spawn
				s.SpawnItem(px, py+1.5, pz, vx, vy, vz, dropItemID, dropDamage, dropCount)
				player.mu.Lock()
			}
		}
	} else if slotNum == -999 && mode == 0 && player.Cursor.ItemID != -1 { // Clicked outside with cursor
		dropCount := player.Cursor.Count
		if button == 0 {
			dropCount = player.Cursor.Count
		} else {
			dropCount = 1
		} // Left drops all, right drops 1

		dropDamage := player.Cursor.Damage
		dropItemID := player.Cursor.ItemID

		player.Cursor.Count -= dropCount
		if player.Cursor.Count <= 0 {
			player.Cursor = Slot{ItemID: -1}
		}

		f1 := math.Sin(float64(player.Yaw) * math.Pi / 180.0)
		f2 := math.Cos(float64(player.Yaw) * math.Pi / 180.0)
		f3 := math.Sin(float64(player.Pitch) * math.Pi / 180.0)
		f4 := math.Cos(float64(player.Pitch) * math.Pi / 180.0)

		vx := -f1 * f4 * 0.3
		vy := -f3*0.3 + 0.1
		vz := f2 * f4 * 0.3

		player.mu.Unlock()
		s.SpawnItem(px, py+1.5, pz, vx, vy, vz, dropItemID, dropDamage, dropCount)
		player.mu.Lock()
	}

	// Update crafting output based on current grid contents
	updateCraftOutput2x2(player)

	// Acknowledge action
	confirmPkt := protocol.MarshalPacket(0x32, func(w *bytes.Buffer) {
		protocol.WriteByte(w, 0) // window ID
		protocol.WriteInt16(w, actionNum)
		protocol.WriteBool(w, true) // accepted
	})

	// Always send a full WindowItems sync and SetSlot for cursor to prevent ANY duplication/desyncs!
	syncPkt := protocol.MarshalPacket(0x30, func(w *bytes.Buffer) {
		protocol.WriteByte(w, 0)   // Window ID
		protocol.WriteInt16(w, 45) // Count
		for i := 0; i < 45; i++ {
			slot := player.Inventory[i]
			protocol.WriteSlotData(w, slot.ItemID, slot.Count, slot.Damage)
		}
	})
	cursorPkt := protocol.MarshalPacket(0x2F, func(w *bytes.Buffer) {
		protocol.WriteByte(w, 0xff) // Cursor
		protocol.WriteInt16(w, -1)
		protocol.WriteSlotData(w, player.Cursor.ItemID, player.Cursor.Count, player.Cursor.Damage)
	})

	if player.Conn != nil {
		protocol.WritePacket(player.Conn, confirmPkt)
		protocol.WritePacket(player.Conn, syncPkt)
		protocol.WritePacket(player.Conn, cursorPkt)
	}

	player.mu.Unlock()
	// After any inventory manipulation, ensure other players see the
	// correct held item for this player.
	s.broadcastHeldItem(player)
}

// handleWindowClick handles Click Window packets for non-player-inventory windows (e.g. crafting table).
func (s *Server) handleWindowClick(player *Player, windowID byte, slotNum int16, button byte, actionNum int16, mode byte) {
	player.mu.Lock()

	if windowID != player.OpenWindowID {
		player.mu.Unlock()
		return
	}

	px, py, pz := player.X, player.Y, player.Z
	totalSlots := int16(46) // 1 output + 9 grid + 27 main + 9 hotbar

	// Slot accessors: translate window slot to storage
	getSlot := func(n int16) Slot {
		switch {
		case n == 0:
			return player.CraftTableOutput
		case n >= 1 && n <= 9:
			return player.CraftTableGrid[n-1]
		case n >= 10 && n <= 45:
			return player.Inventory[n-1]
		}
		return Slot{ItemID: -1}
	}
	setSlot := func(n int16, sl Slot) {
		switch {
		case n == 0:
			player.CraftTableOutput = sl
		case n >= 1 && n <= 9:
			player.CraftTableGrid[n-1] = sl
		case n >= 10 && n <= 45:
			player.Inventory[n-1] = sl
		}
	}

	// Handle crafting output (slot 0) specially
	if slotNum == 0 {
		if mode == 0 && player.CraftTableOutput.ItemID != -1 {
			result := player.CraftTableOutput
			if player.Cursor.ItemID == -1 {
				player.Cursor = result
				consumeCraftIngredients3x3(player)
			} else if player.Cursor.ItemID == result.ItemID && player.Cursor.Damage == result.Damage && int(player.Cursor.Count)+int(result.Count) <= 64 {
				player.Cursor.Count += result.Count
				consumeCraftIngredients3x3(player)
			}
		} else if mode == 1 && player.CraftTableOutput.ItemID != -1 {
			// Shift-click: craft all possible
			for player.CraftTableOutput.ItemID != -1 {
				result := player.CraftTableOutput
				_, ok := addItemToInventory(player, result.ItemID, result.Damage, result.Count)
				if !ok {
					break
				}
				consumeCraftIngredients3x3(player)
				updateCraftOutput3x3(player)
			}
		}
	} else if slotNum >= 1 && slotNum < totalSlots {
		if mode == 0 { // Normal click
			sl := getSlot(slotNum)
			if button == 0 { // Left click
				if player.Cursor.ItemID == sl.ItemID && player.Cursor.Damage == sl.Damage && player.Cursor.ItemID != -1 {
					space := 64 - sl.Count
					if player.Cursor.Count <= space {
						sl.Count += player.Cursor.Count
						player.Cursor = Slot{ItemID: -1}
					} else {
						sl.Count = 64
						player.Cursor.Count -= space
					}
					setSlot(slotNum, sl)
				} else {
					setSlot(slotNum, player.Cursor)
					player.Cursor = sl
				}
			} else if button == 1 { // Right click
				if player.Cursor.ItemID == -1 && sl.ItemID != -1 {
					half := (sl.Count + 1) / 2
					player.Cursor = sl
					player.Cursor.Count = half
					sl.Count -= half
					if sl.Count == 0 {
						sl = Slot{ItemID: -1}
					}
					setSlot(slotNum, sl)
				} else if player.Cursor.ItemID != -1 && sl.ItemID == -1 {
					sl = player.Cursor
					sl.Count = 1
					setSlot(slotNum, sl)
					player.Cursor.Count--
					if player.Cursor.Count == 0 {
						player.Cursor = Slot{ItemID: -1}
					}
				} else if player.Cursor.ItemID == sl.ItemID && player.Cursor.Damage == sl.Damage {
					if sl.Count < 64 {
						sl.Count++
						setSlot(slotNum, sl)
						player.Cursor.Count--
						if player.Cursor.Count == 0 {
							player.Cursor = Slot{ItemID: -1}
						}
					}
				} else {
					setSlot(slotNum, player.Cursor)
					player.Cursor = sl
				}
			}
		} else if mode == 1 { // Shift-click
			sl := getSlot(slotNum)
			if sl.ItemID != -1 {
				var destStart, destEnd int16
				if slotNum >= 1 && slotNum <= 9 {
					destStart, destEnd = 10, 45
				} else if slotNum >= 10 && slotNum <= 36 {
					destStart, destEnd = 37, 45
				} else if slotNum >= 37 && slotNum <= 45 {
					destStart, destEnd = 10, 36
				} else {
					destStart, destEnd = 10, 45
				}
				remaining := sl.Count
				for i := destStart; i <= destEnd && remaining > 0; i++ {
					ds := getSlot(i)
					if ds.ItemID == sl.ItemID && ds.Damage == sl.Damage && ds.Count < 64 {
						space := 64 - ds.Count
						if remaining <= space {
							ds.Count += remaining
							remaining = 0
						} else {
							ds.Count = 64
							remaining -= space
						}
						setSlot(i, ds)
					}
				}
				for i := destStart; i <= destEnd && remaining > 0; i++ {
					ds := getSlot(i)
					if ds.ItemID == -1 {
						setSlot(i, Slot{ItemID: sl.ItemID, Damage: sl.Damage, Count: remaining})
						remaining = 0
					}
				}
				if remaining == 0 {
					setSlot(slotNum, Slot{ItemID: -1})
				} else if remaining < sl.Count {
					sl.Count = remaining
					setSlot(slotNum, sl)
				}
			}
		} else if mode == 2 { // Number key hotkey
			hotbarWinSlot := int16(37) + int16(button)
			if hotbarWinSlot >= 37 && hotbarWinSlot <= 45 {
				temp := getSlot(slotNum)
				setSlot(slotNum, getSlot(hotbarWinSlot))
				setSlot(hotbarWinSlot, temp)
			}
		}
	}

	// Mode 6: double-click collect
	if mode == 6 && player.Cursor.ItemID != -1 {
		for i := int16(1); i < totalSlots && player.Cursor.Count < 64; i++ {
			sl := getSlot(i)
			if sl.ItemID == player.Cursor.ItemID && sl.Damage == player.Cursor.Damage {
				space := 64 - player.Cursor.Count
				if sl.Count <= space {
					player.Cursor.Count += sl.Count
					setSlot(i, Slot{ItemID: -1})
				} else {
					player.Cursor.Count = 64
					sl.Count -= space
					setSlot(i, sl)
				}
			}
		}
	}

	// Mode 5: drag/paint
	if mode == 5 {
		switch button {
		case 0:
			player.DragSlots = nil
			player.DragButton = 0
		case 4:
			player.DragSlots = nil
			player.DragButton = 1
		case 1:
			if slotNum >= 1 && slotNum < totalSlots {
				player.DragSlots = append(player.DragSlots, slotNum)
			}
		case 5:
			if slotNum >= 1 && slotNum < totalSlots {
				player.DragSlots = append(player.DragSlots, slotNum)
			}
		case 2: // Left drag end
			if player.Cursor.ItemID != -1 && len(player.DragSlots) > 0 {
				perSlot := player.Cursor.Count / byte(len(player.DragSlots))
				if perSlot < 1 {
					perSlot = 1
				}
				for _, ds := range player.DragSlots {
					if player.Cursor.Count <= 0 {
						break
					}
					dsl := getSlot(ds)
					if dsl.ItemID == -1 {
						give := perSlot
						if give > player.Cursor.Count {
							give = player.Cursor.Count
						}
						setSlot(ds, Slot{ItemID: player.Cursor.ItemID, Damage: player.Cursor.Damage, Count: give})
						player.Cursor.Count -= give
					} else if dsl.ItemID == player.Cursor.ItemID && dsl.Damage == player.Cursor.Damage {
						space := 64 - dsl.Count
						give := perSlot
						if give > space {
							give = space
						}
						if give > player.Cursor.Count {
							give = player.Cursor.Count
						}
						dsl.Count += give
						setSlot(ds, dsl)
						player.Cursor.Count -= give
					}
				}
				if player.Cursor.Count <= 0 {
					player.Cursor = Slot{ItemID: -1}
				}
			}
			player.DragSlots = nil
		case 6: // Right drag end
			if player.Cursor.ItemID != -1 && len(player.DragSlots) > 0 {
				for _, ds := range player.DragSlots {
					if player.Cursor.Count <= 0 {
						break
					}
					dsl := getSlot(ds)
					if dsl.ItemID == -1 {
						setSlot(ds, Slot{ItemID: player.Cursor.ItemID, Damage: player.Cursor.Damage, Count: 1})
						player.Cursor.Count--
					} else if dsl.ItemID == player.Cursor.ItemID && dsl.Damage == player.Cursor.Damage && dsl.Count < 64 {
						dsl.Count++
						setSlot(ds, dsl)
						player.Cursor.Count--
					}
				}
				if player.Cursor.Count <= 0 {
					player.Cursor = Slot{ItemID: -1}
				}
			}
			player.DragSlots = nil
		}
	}

	// Mode 4: drop
	if mode == 4 && player.GameMode != GameModeSpectator {
		if slotNum == -999 {
			if player.Cursor.ItemID != -1 {
				vitemID := player.Cursor.ItemID
				vdamage := player.Cursor.Damage
				dropCount := player.Cursor.Count
				if button == 0 {
					dropCount = 1
					player.Cursor.Count--
					if player.Cursor.Count <= 0 {
						player.Cursor = Slot{ItemID: -1}
					}
				} else {
					player.Cursor = Slot{ItemID: -1}
				}
				f1 := math.Sin(float64(player.Yaw) * math.Pi / 180.0)
				f2 := math.Cos(float64(player.Yaw) * math.Pi / 180.0)
				f3 := math.Sin(float64(player.Pitch) * math.Pi / 180.0)
				f4 := math.Cos(float64(player.Pitch) * math.Pi / 180.0)
				vx := -f1 * f4 * 0.3
				vy := -f3*0.3 + 0.1
				vz := f2 * f4 * 0.3
				player.mu.Unlock()
				s.SpawnItem(px, py+1.5, pz, vx, vy, vz, vitemID, vdamage, dropCount)
				player.mu.Lock()
			}
		} else if slotNum >= 1 && slotNum < totalSlots {
			sl := getSlot(slotNum)
			if sl.ItemID != -1 {
				dropItemID := sl.ItemID
				dropDamage := sl.Damage
				dropCount := sl.Count
				if button == 0 {
					dropCount = 1
					sl.Count--
					if sl.Count <= 0 {
						sl = Slot{ItemID: -1}
					}
					setSlot(slotNum, sl)
				} else {
					setSlot(slotNum, Slot{ItemID: -1})
				}
				f1 := math.Sin(float64(player.Yaw) * math.Pi / 180.0)
				f2 := math.Cos(float64(player.Yaw) * math.Pi / 180.0)
				f3 := math.Sin(float64(player.Pitch) * math.Pi / 180.0)
				f4 := math.Cos(float64(player.Pitch) * math.Pi / 180.0)
				vx := -f1 * f4 * 0.3
				vy := -f3*0.3 + 0.1
				vz := f2 * f4 * 0.3
				player.mu.Unlock()
				s.SpawnItem(px, py+1.5, pz, vx, vy, vz, dropItemID, dropDamage, dropCount)
				player.mu.Lock()
			}
		}
	} else if slotNum == -999 && mode == 0 && player.Cursor.ItemID != -1 {
		dropCount := player.Cursor.Count
		if button == 1 {
			dropCount = 1
		}
		dropItemID := player.Cursor.ItemID
		dropDamage := player.Cursor.Damage
		player.Cursor.Count -= dropCount
		if player.Cursor.Count <= 0 {
			player.Cursor = Slot{ItemID: -1}
		}
		f1 := math.Sin(float64(player.Yaw) * math.Pi / 180.0)
		f2 := math.Cos(float64(player.Yaw) * math.Pi / 180.0)
		f3 := math.Sin(float64(player.Pitch) * math.Pi / 180.0)
		f4 := math.Cos(float64(player.Pitch) * math.Pi / 180.0)
		vx := -f1 * f4 * 0.3
		vy := -f3*0.3 + 0.1
		vz := f2 * f4 * 0.3
		player.mu.Unlock()
		s.SpawnItem(px, py+1.5, pz, vx, vy, vz, dropItemID, dropDamage, dropCount)
		player.mu.Lock()
	}

	// Update crafting output
	updateCraftOutput3x3(player)

	// Acknowledge action
	confirmPkt := protocol.MarshalPacket(0x32, func(w *bytes.Buffer) {
		protocol.WriteByte(w, windowID)
		protocol.WriteInt16(w, actionNum)
		protocol.WriteBool(w, true)
	})

	// Send full window items sync
	syncPkt := protocol.MarshalPacket(0x30, func(w *bytes.Buffer) {
		protocol.WriteByte(w, windowID)
		protocol.WriteInt16(w, totalSlots)
		// Slot 0: crafting output
		protocol.WriteSlotData(w, player.CraftTableOutput.ItemID, player.CraftTableOutput.Count, player.CraftTableOutput.Damage)
		// Slots 1-9: crafting grid
		for i := 0; i < 9; i++ {
			sl := player.CraftTableGrid[i]
			protocol.WriteSlotData(w, sl.ItemID, sl.Count, sl.Damage)
		}
		// Slots 10-36: main inventory (player slots 9-35)
		for i := 9; i <= 35; i++ {
			sl := player.Inventory[i]
			protocol.WriteSlotData(w, sl.ItemID, sl.Count, sl.Damage)
		}
		// Slots 37-45: hotbar (player slots 36-44)
		for i := 36; i <= 44; i++ {
			sl := player.Inventory[i]
			protocol.WriteSlotData(w, sl.ItemID, sl.Count, sl.Damage)
		}
	})
	cursorPkt := protocol.MarshalPacket(0x2F, func(w *bytes.Buffer) {
		protocol.WriteByte(w, 0xff)
		protocol.WriteInt16(w, -1)
		protocol.WriteSlotData(w, player.Cursor.ItemID, player.Cursor.Count, player.Cursor.Damage)
	})

	if player.Conn != nil {
		protocol.WritePacket(player.Conn, confirmPkt)
		protocol.WritePacket(player.Conn, syncPkt)
		protocol.WritePacket(player.Conn, cursorPkt)
	}

	player.mu.Unlock()
	s.broadcastHeldItem(player)
}
