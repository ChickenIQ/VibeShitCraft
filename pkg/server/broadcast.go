package server

import (
	"bytes"

	"github.com/StoreStation/VibeShitCraft/pkg/chat"
	"github.com/StoreStation/VibeShitCraft/pkg/protocol"
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

func (s *Server) broadcastBlockBreakEffect(breaker *Player, x, y, z int32, blockState uint16) {
	blockID := blockState >> 4
	metadata := blockState & 0x0F
	effectData := int32(blockID) | (int32(metadata) << 12)

	pkt := protocol.MarshalPacket(0x28, func(w *bytes.Buffer) {
		protocol.WriteInt32(w, 2001) // Effect ID: block break
		protocol.WritePosition(w, x, y, z)
		protocol.WriteInt32(w, effectData)
		protocol.WriteBool(w, false) // Disable relative volume
	})

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.players {
		if p.EntityID == breaker.EntityID {
			continue // Breaking player already sees the effect client-side
		}
		p.mu.Lock()
		if p.Conn != nil {
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
		if p.Conn != nil {
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
		if p.Conn != nil {
			protocol.WritePacket(p.Conn, pkt)
		}
		p.mu.Unlock()
	}
}

func (s *Server) broadcastEntityTeleport(player *Player) {
	player.mu.Lock()
	x := player.X
	y := player.Y
	z := player.Z
	yaw := player.Yaw
	pitch := player.Pitch
	onGround := player.OnGround
	player.mu.Unlock()

	pkt := protocol.MarshalPacket(0x18, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, player.EntityID)
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
		if other.EntityID == player.EntityID {
			continue
		}
		other.mu.Lock()
		protocol.WritePacket(other.Conn, pkt)
		other.mu.Unlock()
	}
}

func (s *Server) broadcastEntityLook(player *Player) {
	player.mu.Lock()
	yaw := player.Yaw
	pitch := player.Pitch
	onGround := player.OnGround
	player.mu.Unlock()

	pkt := protocol.MarshalPacket(0x16, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, player.EntityID)
		protocol.WriteByte(w, byte(yaw*256/360))
		protocol.WriteByte(w, byte(pitch*256/360))
		protocol.WriteBool(w, onGround)
	})

	headRotation := protocol.MarshalPacket(0x19, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, player.EntityID)
		protocol.WriteByte(w, byte(yaw*256/360))
	})

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, other := range s.players {
		if other.EntityID == player.EntityID {
			continue
		}
		other.mu.Lock()
		protocol.WritePacket(other.Conn, pkt)
		protocol.WritePacket(other.Conn, headRotation)
		other.mu.Unlock()
	}
}

func (s *Server) broadcastAnimation(player *Player, animationID byte) {
	pkt := protocol.MarshalPacket(0x0B, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, player.EntityID)
		protocol.WriteByte(w, animationID)
	})

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, other := range s.players {
		if other.EntityID == player.EntityID {
			continue
		}
		other.mu.Lock()
		protocol.WritePacket(other.Conn, pkt)
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
			protocol.WritePacket(p.Conn, pkt)
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
		if p.Conn != nil {
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
	if activeSlot < 0 || slotIndex < 0 || slotIndex >= len(player.Inventory) {
		player.mu.Unlock()
		return
	}
	slot := player.Inventory[slotIndex]
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
		if other.Conn != nil {
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

	for _, other := range s.players {
		if other.EntityID == player.EntityID {
			continue
		}
		s.sendSpawnPlayer(other, player)
	}
}

func (s *Server) spawnOthersForPlayer(player *Player) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, other := range s.players {
		if other.EntityID == player.EntityID {
			continue
		}
		s.sendSpawnPlayer(player, other)
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
	protocol.WritePacket(viewer.Conn, playerListAdd)
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
	protocol.WritePacket(viewer.Conn, spawnPlayer)
	viewer.mu.Unlock()
}
