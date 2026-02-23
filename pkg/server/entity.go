package server

import (
	"bytes"
	"log"
	"math"
	"time"

	"github.com/StoreStation/VibeShitCraft/pkg/protocol"
)

// ItemEntity represents an item dropped on the ground.
type ItemEntity struct {
	EntityID   int32
	ItemID     int16
	Damage     int16
	Count      byte
	X, Y, Z    float64
	VX, VY, VZ float64 // Velocity tracking for drops
	SpawnTime  time.Time
}

// MobEntity represents a living entity (mob) in the world.
type MobEntity struct {
	EntityID   int32
	MobType    byte // Minecraft mob type ID (e.g., 50=Creeper, 90=Pig)
	X, Y, Z    float64
	VX, VY, VZ float64
	Yaw, Pitch float32
	HeadPitch  float32
	OnGround   bool
	// AIFunc is an optional AI callback invoked each tick. Can be nil.
	AIFunc func(mob *MobEntity, s *Server)
}

func (s *Server) entityPhysicsLoop() {
	ticker := time.NewTicker(50 * time.Millisecond) // 20 ticks per second
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.tickEntityPhysics()
		}
	}
}

func (s *Server) tickEntityPhysics() {
	const gravity = 0.04
	const drag = 0.98
	const groundDrag = 0.5

	s.mu.Lock()

	// Collect entities that moved so we can broadcast after releasing the lock
	type movedItem struct {
		entityID int32
		x, y, z  float64
	}
	var movedItems []movedItem

	for _, item := range s.entities {
		if item.VX == 0 && item.VY == 0 && item.VZ == 0 {
			// Check if entity is supported by ground
			blockBelow := s.world.GetBlock(int32(math.Floor(item.X)), int32(math.Floor(item.Y-0.1)), int32(math.Floor(item.Z)))
			if blockBelow>>4 != 0 {
				continue
			}
			// Not on ground and no velocity - start falling
		}

		// Apply gravity
		item.VY -= gravity

		// Apply drag
		item.VX *= drag
		item.VY *= drag
		item.VZ *= drag

		// Calculate new position
		newX := item.X + item.VX
		newY := item.Y + item.VY
		newZ := item.Z + item.VZ

		// Ground collision check
		blockAtNew := s.world.GetBlock(int32(math.Floor(newX)), int32(math.Floor(newY)), int32(math.Floor(newZ)))
		if blockAtNew>>4 != 0 {
			// Hit solid block - place on top
			newY = math.Floor(newY) + 1.0
			item.VY = 0
			item.VX *= groundDrag
			item.VZ *= groundDrag

			// Stop tiny velocities
			if math.Abs(item.VX) < 0.001 {
				item.VX = 0
			}
			if math.Abs(item.VZ) < 0.001 {
				item.VZ = 0
			}
		}

		item.X = newX
		item.Y = newY
		item.Z = newZ

		movedItems = append(movedItems, movedItem{item.EntityID, item.X, item.Y, item.Z})
	}

	// Tick mob entities: gravity, physics, and optional AI
	type movedMob struct {
		entityID   int32
		x, y, z    float64
		yaw, pitch float32
		onGround   bool
	}
	var movedMobs []movedMob

	for _, mob := range s.mobEntities {
		// Run AI if present
		if mob.AIFunc != nil {
			mob.AIFunc(mob, s)
		}

		if mob.VX == 0 && mob.VY == 0 && mob.VZ == 0 {
			blockBelow := s.world.GetBlock(int32(math.Floor(mob.X)), int32(math.Floor(mob.Y-0.1)), int32(math.Floor(mob.Z)))
			if blockBelow>>4 != 0 {
				mob.OnGround = true
				continue
			}
			mob.OnGround = false
		}

		// Apply gravity
		mob.VY -= gravity

		// Apply drag
		mob.VX *= drag
		mob.VY *= drag
		mob.VZ *= drag

		newX := mob.X + mob.VX
		newY := mob.Y + mob.VY
		newZ := mob.Z + mob.VZ

		// Ground collision
		blockAtNew := s.world.GetBlock(int32(math.Floor(newX)), int32(math.Floor(newY)), int32(math.Floor(newZ)))
		if blockAtNew>>4 != 0 {
			newY = math.Floor(newY) + 1.0
			mob.VY = 0
			mob.VX *= groundDrag
			mob.VZ *= groundDrag
			mob.OnGround = true

			if math.Abs(mob.VX) < 0.001 {
				mob.VX = 0
			}
			if math.Abs(mob.VZ) < 0.001 {
				mob.VZ = 0
			}
		} else {
			mob.OnGround = false
		}

		mob.X = newX
		mob.Y = newY
		mob.Z = newZ

		movedMobs = append(movedMobs, movedMob{mob.EntityID, mob.X, mob.Y, mob.Z, mob.Yaw, mob.Pitch, mob.OnGround})
	}

	s.mu.Unlock()

	// Broadcast entity teleport packets
	for _, m := range movedItems {
		s.broadcastEntityTeleportByID(m.entityID, m.x, m.y, m.z, 0, 0, true)
	}
	for _, m := range movedMobs {
		s.broadcastEntityTeleportByID(m.entityID, m.x, m.y, m.z, m.yaw, m.pitch, m.onGround)
	}
}

// SpawnItem creates an item entity at the given position and broadcasts it.
func (s *Server) SpawnItem(x, y, z float64, vx, vy, vz float64, itemID int16, damage int16, count byte) {
	s.mu.Lock()
	eid := s.nextEID
	s.nextEID++

	item := &ItemEntity{
		EntityID:  eid,
		ItemID:    itemID,
		Damage:    damage,
		Count:     count,
		X:         x,
		Y:         y,
		Z:         z,
		VX:        vx,
		VY:        vy,
		VZ:        vz,
		SpawnTime: time.Now(),
	}
	s.entities[eid] = item
	s.mu.Unlock()

	s.broadcastSpawnItem(item)
}

func (s *Server) broadcastSpawnItem(item *ItemEntity) {
	// Spawn Object - 0x0E (Item Stack)
	spawnObj := protocol.MarshalPacket(0x0E, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, item.EntityID)
		protocol.WriteByte(w, 2) // Type: Item Stack
		protocol.WriteInt32(w, int32(item.X*32))
		protocol.WriteInt32(w, int32(item.Y*32))
		protocol.WriteInt32(w, int32(item.Z*32))
		protocol.WriteByte(w, 0)  // Pitch
		protocol.WriteByte(w, 0)  // Yaw
		protocol.WriteInt32(w, 0) // Data for thrower (0 usually)
	})

	// Entity Velocity - 0x12
	velocityPkt := protocol.MarshalPacket(0x12, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, item.EntityID)
		protocol.WriteInt16(w, int16(item.VX*8000))
		protocol.WriteInt16(w, int16(item.VY*8000))
		protocol.WriteInt16(w, int16(item.VZ*8000))
	})

	// Entity Metadata - 0x1C
	metadata := protocol.MarshalPacket(0x1C, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, item.EntityID)
		// Metadata for item stack (index 10, type 5: Slot)
		// Header byte: (type << 5) | (index & 0x1F)
		protocol.WriteByte(w, (5<<5)|10)
		protocol.WriteSlotData(w, item.ItemID, item.Count, item.Damage)
		protocol.WriteByte(w, 0x7F) // Terminator
	})

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.players {
		p.mu.Lock()
		if p.Conn != nil {
			protocol.WritePacket(p.Conn, spawnObj)
			protocol.WritePacket(p.Conn, velocityPkt)
			protocol.WritePacket(p.Conn, metadata)
		}
		p.mu.Unlock()
	}
}

// SpawnMob creates a mob entity at the given position and broadcasts it to all players.
func (s *Server) SpawnMob(x, y, z float64, mobType byte) {
	s.mu.Lock()
	eid := s.nextEID
	s.nextEID++

	mob := &MobEntity{
		EntityID: eid,
		MobType:  mobType,
		X:        x,
		Y:        y,
		Z:        z,
	}
	s.mobEntities[eid] = mob
	s.mu.Unlock()

	s.broadcastSpawnMob(mob)
	log.Printf("Spawned mob type %d (EID: %d) at (%.1f, %.1f, %.1f)", mobType, eid, x, y, z)
}

func (s *Server) broadcastSpawnMob(mob *MobEntity) {
	// Spawn Mob packet - 0x0F
	pkt := protocol.MarshalPacket(0x0F, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, mob.EntityID)
		protocol.WriteByte(w, mob.MobType)
		protocol.WriteInt32(w, int32(mob.X*32))
		protocol.WriteInt32(w, int32(mob.Y*32))
		protocol.WriteInt32(w, int32(mob.Z*32))
		protocol.WriteByte(w, byte(mob.Yaw*256/360))
		protocol.WriteByte(w, byte(mob.Pitch*256/360))
		protocol.WriteByte(w, byte(mob.HeadPitch*256/360))
		protocol.WriteInt16(w, int16(mob.VX*8000))
		protocol.WriteInt16(w, int16(mob.VY*8000))
		protocol.WriteInt16(w, int16(mob.VZ*8000))
		protocol.WriteByte(w, 0x7F) // Metadata terminator (no extra metadata)
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

func (s *Server) spawnEntitiesForPlayer(player *Player) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, entity := range s.entities {
		s.sendItemToPlayer(player, entity)
	}
}

func (s *Server) sendItemToPlayer(player *Player, item *ItemEntity) {
	// Spawn Object - 0x0E (Item Stack)
	spawnObj := protocol.MarshalPacket(0x0E, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, item.EntityID)
		protocol.WriteByte(w, 2) // Type: Item Stack
		protocol.WriteInt32(w, int32(item.X*32))
		protocol.WriteInt32(w, int32(item.Y*32))
		protocol.WriteInt32(w, int32(item.Z*32))
		protocol.WriteByte(w, 0)  // Pitch
		protocol.WriteByte(w, 0)  // Yaw
		protocol.WriteInt32(w, 0) // Data (needs to be non-zero for some objects, but 0 often means no velocity)
	})

	// Entity Metadata - 0x1C
	metadata := protocol.MarshalPacket(0x1C, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, item.EntityID)
		// Metadata for item stack (index 10, type 5: Slot)
		// Header byte: (type << 5) | (index & 0x1F)
		protocol.WriteByte(w, (5<<5)|10)
		protocol.WriteSlotData(w, item.ItemID, item.Count, item.Damage)
		protocol.WriteByte(w, 0x7F) // Terminator
	})

	player.mu.Lock()
	protocol.WritePacket(player.Conn, spawnObj)
	protocol.WritePacket(player.Conn, metadata)
	player.mu.Unlock()
}

func (s *Server) spawnMobEntitiesForPlayer(player *Player) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, mob := range s.mobEntities {
		s.sendMobToPlayer(player, mob)
	}
}

func (s *Server) sendMobToPlayer(player *Player, mob *MobEntity) {
	pkt := protocol.MarshalPacket(0x0F, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, mob.EntityID)
		protocol.WriteByte(w, mob.MobType)
		protocol.WriteInt32(w, int32(mob.X*32))
		protocol.WriteInt32(w, int32(mob.Y*32))
		protocol.WriteInt32(w, int32(mob.Z*32))
		protocol.WriteByte(w, byte(mob.Yaw*256/360))
		protocol.WriteByte(w, byte(mob.Pitch*256/360))
		protocol.WriteByte(w, byte(mob.HeadPitch*256/360))
		protocol.WriteInt16(w, int16(mob.VX*8000))
		protocol.WriteInt16(w, int16(mob.VY*8000))
		protocol.WriteInt16(w, int16(mob.VZ*8000))
		protocol.WriteByte(w, 0x7F) // Metadata terminator
	})

	player.mu.Lock()
	if player.Conn != nil {
		protocol.WritePacket(player.Conn, pkt)
	}
	player.mu.Unlock()
}

func (s *Server) broadcastCollectItem(collectedID, collectorID int32) {
	pkt := protocol.MarshalPacket(0x0D, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, collectedID)
		protocol.WriteVarInt(w, collectorID)
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

func (s *Server) itemPickupLoop(player *Player, stop chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			s.mu.RLock()
			if len(s.entities) == 0 {
				s.mu.RUnlock()
				continue
			}
			// Copy entities to avoid long lock
			entities := make([]*ItemEntity, 0, len(s.entities))
			for _, e := range s.entities {
				entities = append(entities, e)
			}
			s.mu.RUnlock()

			player.mu.Lock()
			px, py, pz := player.X, player.Y, player.Z
			isDead := player.IsDead
			player.mu.Unlock()

			if isDead {
				continue
			}

			for _, e := range entities {
				// Skip recently spawned items (1 second pickup delay)
				if time.Since(e.SpawnTime) < 1*time.Second {
					continue
				}

				dx := e.X - px
				dy := e.Y - py
				dz := e.Z - pz
				distSq := dx*dx + dy*dy + dz*dz

				if distSq < 4.0 { // 2.0 blocks range
					// Try to pick up
					player.mu.Lock()
					slotIndex, ok := addItemToInventory(player, e.ItemID, e.Damage, e.Count)
					if ok {
						// Success!
						slot := player.Inventory[slotIndex]
						pkt := protocol.MarshalPacket(0x2F, func(w *bytes.Buffer) {
							protocol.WriteByte(w, 0) // Window ID 0 = player inventory
							protocol.WriteInt16(w, int16(slotIndex))
							protocol.WriteSlotData(w, slot.ItemID, slot.Count, slot.Damage)
						})
						if player.Conn != nil {
							protocol.WritePacket(player.Conn, pkt)
						}
						player.mu.Unlock()

						// Remove entity
						s.mu.Lock()
						// Re-check if it's still there (some other player might have picked it up)
						if _, exists := s.entities[e.EntityID]; exists {
							delete(s.entities, e.EntityID)
							s.mu.Unlock()

							// Broadcast collect and destroy
							s.broadcastCollectItem(e.EntityID, player.EntityID)
							s.broadcastDestroyEntity(e.EntityID)
							log.Printf("Player %s picked up item %d:%d", player.Username, e.ItemID, e.Damage)
							// Picking up an item may have filled or stacked the
							// currently selected hotbar slot; notify others so
							// they see the updated held item.
							s.broadcastHeldItem(player)
						} else {
							s.mu.Unlock()
						}
						continue
					}
					player.mu.Unlock()
				}
			}
		}
	}
}
