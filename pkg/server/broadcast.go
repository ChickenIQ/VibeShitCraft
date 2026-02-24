package server

import (
	"bytes"

	"github.com/VibeShit/VibeShitCraft/pkg/chat"
	"github.com/VibeShit/VibeShitCraft/pkg/protocol"
)

func (s *Server) broadcastChat(msg chat.Message) {
	jsonMsg := msg.String()
	pkt := protocol.MarshalPacket(0x02, func(w *bytes.Buffer) {
		protocol.WriteString(w, jsonMsg)
		protocol.WriteByte(w, 0) // Position: chat
	})

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.players {
		p.mu.Lock()
		if p.Conn != nil {
			protocol.WritePacket(p.Conn, pkt)
		}
		p.mu.Unlock()
	}
}

// sendChatToPlayer sends a chat message to a single player.
func (s *Server) sendChatToPlayer(player *Player, msg chat.Message) {
	jsonMsg := msg.String()
	pkt := protocol.MarshalPacket(0x02, func(w *bytes.Buffer) {
		protocol.WriteString(w, jsonMsg)
		protocol.WriteByte(w, 0) // Position: chat
	})
	player.mu.Lock()
	protocol.WritePacket(player.Conn, pkt)
	player.mu.Unlock()
}

func (s *Server) broadcastBlockChange(x, y, z int32, blockState uint16) {
	pkt := protocol.MarshalPacket(0x23, func(w *bytes.Buffer) {
		protocol.WritePosition(w, x, y, z)
		protocol.WriteVarInt(w, int32(blockState))
	})

	pos := ChunkPos{x >> 4, z >> 4}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.players {
		p.mu.Lock()
		if p.Conn != nil && p.loadedChunks[pos] {
			protocol.WritePacket(p.Conn, pkt)
		}
		p.mu.Unlock()
	}
}

func (s *Server) broadcastBlockBreakEffect(breaker *Player, x, y, z int32, blockState uint16) {
	blockID := blockState >> 4
	metadata := blockState & 0x0F

	// Wheat crops (59) with metadata > 0 can cause weird particle colors (like emerald blocks)
	// when broken by other players, due to how the 1.8 client handles Effect 2001.
	// To fix this, we strip the metadata for crops.
	if blockID == 59 {
		metadata = 0
	}

	effectData := int32(blockID) | (int32(metadata) << 12)

	pkt := protocol.MarshalPacket(0x28, func(w *bytes.Buffer) {
		protocol.WriteInt32(w, 2001) // Effect ID: block break
		protocol.WritePosition(w, x, y, z)
		protocol.WriteInt32(w, effectData)
		protocol.WriteBool(w, false) // Disable relative volume
	})

	pos := ChunkPos{x >> 4, z >> 4}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.players {
		if p.EntityID == breaker.EntityID {
			continue // Breaking player already sees the effect client-side
		}
		p.mu.Lock()
		if p.Conn != nil && p.loadedChunks[pos] {
			protocol.WritePacket(p.Conn, pkt)
		}
		p.mu.Unlock()
	}
}

func (s *Server) broadcastEntityTeleportByID(entityID int32, x, y, z float64, yaw, pitch float32, onGround bool) {
	pkt := protocol.MarshalPacket(0x18, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, entityID)
		protocol.WriteInt32(w, int32(x*32))
		protocol.WriteInt32(w, int32(y*32))
		protocol.WriteInt32(w, int32(z*32))
		protocol.WriteByte(w, byte(yaw*256/360))
		protocol.WriteByte(w, byte(pitch*256/360))
		protocol.WriteBool(w, onGround)
	})

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.players {
		p.mu.Lock()
		if p.Conn != nil && p.trackedEntities[entityID] {
			protocol.WritePacket(p.Conn, pkt)
		}
		p.mu.Unlock()
	}
}

func (s *Server) broadcastEntityVelocity(entityID int32, vx, vy, vz float64) {
	pkt := protocol.MarshalPacket(0x12, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, entityID)
		protocol.WriteInt16(w, int16(vx*8000))
		protocol.WriteInt16(w, int16(vy*8000))
		protocol.WriteInt16(w, int16(vz*8000))
	})

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.players {
		p.mu.Lock()
		if p.Conn != nil && p.trackedEntities[entityID] {
			protocol.WritePacket(p.Conn, pkt)
		}
		p.mu.Unlock()
	}
}

func (s *Server) broadcastEntityTeleport(player *Player) {
	player.mu.Lock()
	entityID := player.EntityID
	x := player.X
	y := player.Y
	z := player.Z
	yaw := player.Yaw
	pitch := player.Pitch
	onGround := player.OnGround
	player.mu.Unlock()

	pkt := protocol.MarshalPacket(0x18, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, entityID)
		protocol.WriteInt32(w, int32(x*32))
		protocol.WriteInt32(w, int32(y*32))
		protocol.WriteInt32(w, int32(z*32))
		protocol.WriteByte(w, byte(yaw*256/360))
		protocol.WriteByte(w, byte(pitch*256/360))
		protocol.WriteBool(w, onGround)
	})

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, other := range s.players {
		if other.EntityID == entityID {
			continue
		}
		other.mu.Lock()
		if other.trackedEntities[entityID] {
			protocol.WritePacket(other.Conn, pkt)
		}
		other.mu.Unlock()
	}
}

func (s *Server) broadcastEntityLook(player *Player) {
	player.mu.Lock()
	entityID := player.EntityID
	yaw := player.Yaw
	pitch := player.Pitch
	onGround := player.OnGround
	player.mu.Unlock()

	pkt := protocol.MarshalPacket(0x16, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, entityID)
		protocol.WriteByte(w, byte(yaw*256/360))
		protocol.WriteByte(w, byte(pitch*256/360))
		protocol.WriteBool(w, onGround)
	})

	headRotation := protocol.MarshalPacket(0x19, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, entityID)
		protocol.WriteByte(w, byte(yaw*256/360))
	})

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, other := range s.players {
		if other.EntityID == entityID {
			continue
		}
		other.mu.Lock()
		if other.trackedEntities[entityID] {
			protocol.WritePacket(other.Conn, pkt)
			protocol.WritePacket(other.Conn, headRotation)
		}
		other.mu.Unlock()
	}
}

func (s *Server) broadcastAnimation(player *Player, animationID byte) {
	player.mu.Lock()
	entityID := player.EntityID
	player.mu.Unlock()

	pkt := protocol.MarshalPacket(0x0B, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, entityID)
		protocol.WriteByte(w, animationID)
	})

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, other := range s.players {
		if other.EntityID == entityID {
			continue
		}
		other.mu.Lock()
		if other.trackedEntities[entityID] {
			protocol.WritePacket(other.Conn, pkt)
		}
		other.mu.Unlock()
	}
}

func (s *Server) broadcastDestroyEntity(entityID int32) {
	pkt := protocol.MarshalPacket(0x13, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, 1) // Count
		protocol.WriteVarInt(w, entityID)
	})

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.players {
		p.mu.Lock()
		if p.Conn != nil {
			if p.trackedEntities[entityID] {
				protocol.WritePacket(p.Conn, pkt)
			}
			delete(p.trackedEntities, entityID)
		}
		p.mu.Unlock()
	}
}

func (s *Server) broadcastEntityStatus(entityID int32, status byte) {
	pkt := protocol.MarshalPacket(0x1A, func(w *bytes.Buffer) {
		protocol.WriteInt32(w, entityID)
		protocol.WriteByte(w, status)
	})

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.players {
		p.mu.Lock()
		if p.Conn != nil && p.trackedEntities[entityID] {
			protocol.WritePacket(p.Conn, pkt)
		}
		p.mu.Unlock()
	}
}

// broadcastHeldItem sends an Entity Equipment packet (0x04) to all other
// players so they see the correct item in the given player's hand.
func (s *Server) broadcastHeldItem(player *Player) {
	player.mu.Lock()
	entityID := player.EntityID
	activeSlot := int(player.ActiveSlot)
	slotIndex := 36 + activeSlot
	var slot Slot
	if activeSlot >= 0 && slotIndex >= 0 && slotIndex < len(player.Inventory) {
		slot = player.Inventory[slotIndex]
	} else {
		player.mu.Unlock()
		return
	}
	player.mu.Unlock()

	pkt := protocol.MarshalPacket(0x04, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, entityID)
		// Slot 0 = item in hand in Minecraft 1.8
		protocol.WriteInt16(w, 0)
		protocol.WriteSlotData(w, slot.ItemID, slot.Count, slot.Damage)
	})

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, other := range s.players {
		if other.EntityID == entityID {
			continue
		}
		other.mu.Lock()
		if other.Conn != nil && other.trackedEntities[entityID] {
			protocol.WritePacket(other.Conn, pkt)
		}
		other.mu.Unlock()
	}
}

// broadcastPlayerListRemove sends a Player List Item (action=4, Remove Player)
// to all connected players, removing the target player from the tab list.
func (s *Server) broadcastPlayerListRemove(uuid [16]byte) {
	pkt := protocol.MarshalPacket(0x38, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, 4) // Action: Remove Player
		protocol.WriteVarInt(w, 1) // Number of players
		protocol.WriteUUID(w, uuid)
	})

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.players {
		p.mu.Lock()
		if p.Conn != nil {
			protocol.WritePacket(p.Conn, pkt)
		}
		p.mu.Unlock()
	}
}

func (s *Server) spawnPlayerForOthers(player *Player) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	player.mu.Lock()
	px, py, pz := player.X, player.Y, player.Z
	eid := player.EntityID
	player.mu.Unlock()

	for _, other := range s.players {
		if other.EntityID == eid {
			continue
		}
		if s.shouldTrack(other, px, py, pz) {
			s.sendSpawnPlayer(other, player)
			other.mu.Lock()
			other.trackedEntities[eid] = true
			other.mu.Unlock()
		}
	}
}

func (s *Server) spawnOthersForPlayer(player *Player) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, other := range s.players {
		if other.EntityID == player.EntityID {
			continue
		}
		other.mu.Lock()
		ox, oy, oz := other.X, other.Y, other.Z
		oeid := other.EntityID
		other.mu.Unlock()

		if s.shouldTrack(player, ox, oy, oz) {
			s.sendSpawnPlayer(player, other)
			player.mu.Lock()
			player.trackedEntities[oeid] = true
			player.mu.Unlock()
		}
	}
}

func (s *Server) sendSpawnPlayer(viewer *Player, target *Player) {
	target.mu.Lock()
	x := target.X
	y := target.Y
	z := target.Z
	yaw := target.Yaw
	pitch := target.Pitch
	targetGameMode := target.GameMode
	// Determine the item ID currently held in the target's hand so the viewer
	// immediately sees the correct held item model.
	currentItemID := int16(0)
	activeSlot := int(target.ActiveSlot)
	slotIndex := 36 + activeSlot
	if activeSlot >= 0 && slotIndex >= 0 && slotIndex < len(target.Inventory) {
		held := target.Inventory[slotIndex]
		if held.ItemID > 0 {
			currentItemID = held.ItemID
		}
	}
	target.mu.Unlock()

	// Player List Item (Add Player) - 0x38
	playerListAdd := protocol.MarshalPacket(0x38, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, 0) // Action: Add Player
		protocol.WriteVarInt(w, 1) // Number of players
		protocol.WriteUUID(w, target.UUID)
		protocol.WriteString(w, target.Username)
		protocol.WriteVarInt(w, 0)                      // Number of properties
		protocol.WriteVarInt(w, int32(target.GameMode)) // Gamemode
		protocol.WriteVarInt(w, 0)                      // Ping
		protocol.WriteBool(w, false)                    // Has display name
	})
	viewer.mu.Lock()
	if viewer.Conn != nil {
		protocol.WritePacket(viewer.Conn, playerListAdd)
	}
	viewer.mu.Unlock()

	// Entity flags: set invisible for spectators
	var entityFlags byte
	if targetGameMode == GameModeSpectator {
		entityFlags = EntityFlagInvisible
	}

	// Spawn Player - 0x0C
	spawnPlayer := protocol.MarshalPacket(0x0C, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, target.EntityID)
		protocol.WriteUUID(w, target.UUID)
		protocol.WriteInt32(w, int32(x*32)) // Fixed-point X
		protocol.WriteInt32(w, int32(y*32)) // Fixed-point Y
		protocol.WriteInt32(w, int32(z*32)) // Fixed-point Z
		protocol.WriteByte(w, byte(yaw*256/360))
		protocol.WriteByte(w, byte(pitch*256/360))
		// Current item in hand (ID, not slot index)
		protocol.WriteInt16(w, currentItemID)
		// Minimal entity metadata so clients always receive a non-empty
		// DataWatcher list for spawned players:
		// Index 0, type 0 (byte) = entity flags
		protocol.WriteByte(w, 0x00)        // header: (type 0 << 5) | index 0
		protocol.WriteByte(w, entityFlags) // flags (EntityFlagInvisible for spectators)
		protocol.WriteByte(w, 0x7F)        // Metadata terminator
	})
	viewer.mu.Lock()
	if viewer.Conn != nil {
		protocol.WritePacket(viewer.Conn, spawnPlayer)
	}
	viewer.mu.Unlock()
}

func (s *Server) sendTabComplete(player *Player, matches []string) {
	pkt := protocol.MarshalPacket(0x3A, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, int32(len(matches)))
		for _, match := range matches {
			protocol.WriteString(w, match)
		}
	})
	player.mu.Lock()
	if player.Conn != nil {
		protocol.WritePacket(player.Conn, pkt)
	}
	player.mu.Unlock()
}

// shouldTrack returns true if the entity at (ex, ey, ez) should be visible to the viewer.
func (s *Server) shouldTrack(viewer *Player, ex, ey, ez float64) bool {
	viewer.mu.Lock()
	vx, vy, vz := viewer.X, viewer.Y, viewer.Z
	viewer.mu.Unlock()

	dx := vx - ex
	dy := vy - ey
	dz := vz - ez
	return (dx*dx + dy*dy + dz*dz) <= (EntityTrackingRange * EntityTrackingRange)
}

// updateEntityTracking updates which entities are visible to the given player.
func (s *Server) updateEntityTracking(player *Player) {
	s.mu.RLock()
	players := make([]*Player, 0, len(s.players))
	for _, p := range s.players {
		players = append(players, p)
	}
	items := make([]*ItemEntity, 0, len(s.entities))
	for _, e := range s.entities {
		items = append(items, e)
	}
	mobs := make([]*MobEntity, 0, len(s.mobEntities))
	for _, m := range s.mobEntities {
		mobs = append(mobs, m)
	}
	s.mu.RUnlock()

	// Check players
	for _, other := range players {
		if other.EntityID == player.EntityID {
			continue
		}

		other.mu.Lock()
		ox, oy, oz := other.X, other.Y, other.Z
		oeid := other.EntityID
		other.mu.Unlock()

		player.mu.Lock()
		tracking := player.trackedEntities[oeid]
		player.mu.Unlock()

		shouldTrack := s.shouldTrack(player, ox, oy, oz)

		if shouldTrack && !tracking {
			s.sendSpawnPlayer(player, other)
			player.mu.Lock()
			player.trackedEntities[oeid] = true
			player.mu.Unlock()
		} else if !shouldTrack && tracking {
			s.sendDestroyEntity(player, oeid)
			player.mu.Lock()
			delete(player.trackedEntities, oeid)
			player.mu.Unlock()
		}
	}

	// Check items
	for _, item := range items {
		player.mu.Lock()
		tracking := player.trackedEntities[item.EntityID]
		player.mu.Unlock()

		shouldTrack := s.shouldTrack(player, item.X, item.Y, item.Z)

		if shouldTrack && !tracking {
			s.sendItemToPlayer(player, item)
			player.mu.Lock()
			player.trackedEntities[item.EntityID] = true
			player.mu.Unlock()
		} else if !shouldTrack && tracking {
			s.sendDestroyEntity(player, item.EntityID)
			player.mu.Lock()
			delete(player.trackedEntities, item.EntityID)
			player.mu.Unlock()
		}
	}

	// Check mobs
	for _, mob := range mobs {
		player.mu.Lock()
		tracking := player.trackedEntities[mob.EntityID]
		player.mu.Unlock()

		shouldTrack := s.shouldTrack(player, mob.X, mob.Y, mob.Z)

		if shouldTrack && !tracking {
			s.sendMobToPlayer(player, mob)
			player.mu.Lock()
			player.trackedEntities[mob.EntityID] = true
			player.mu.Unlock()
		} else if !shouldTrack && tracking {
			s.sendDestroyEntity(player, mob.EntityID)
			player.mu.Lock()
			delete(player.trackedEntities, mob.EntityID)
			player.mu.Unlock()
		}
	}
}

func (s *Server) sendDestroyEntity(player *Player, entityID int32) {
	pkt := protocol.MarshalPacket(0x13, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, 1) // Count
		protocol.WriteVarInt(w, entityID)
	})
	player.mu.Lock()
	if player.Conn != nil {
		protocol.WritePacket(player.Conn, pkt)
	}
	player.mu.Unlock()
}
