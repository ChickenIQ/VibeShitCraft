package server

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/StoreStation/VibeShitCraft/pkg/chat"
	"github.com/StoreStation/VibeShitCraft/pkg/protocol"
)

// Slot represents an inventory slot.
type Slot struct {
	ItemID int16
	Count  byte
	Damage int16
}

// Player represents a connected player.
type Player struct {
	EntityID         int32
	Username         string
	UUID             [16]byte
	Conn             net.Conn
	State            int
	GameMode         byte
	X, Y, Z          float64
	Yaw              float32
	Pitch            float32
	OnGround         bool
	Health           float32
	IsDead           bool
	Inventory        [45]Slot
	Cursor           Slot
	ActiveSlot       int16
	loadedChunks     map[ChunkPos]bool
	lastChunkX       int32
	lastChunkZ       int32
	CraftTableGrid   [9]Slot // 3x3 crafting grid for crafting table window
	CraftTableOutput Slot    // Crafting output for crafting table window
	OpenWindowID     byte    // Currently open window ID (0 = none/player inventory)
	NoClip           bool    // True when in spectator mode (can pass through blocks)
	DragSlots        []int16 // Slots being dragged over in mode 5
	DragButton       int     // 0=left drag, 1=right drag
	mu               sync.Mutex
}

func (s *Server) handleLoginStart(conn net.Conn, pkt *protocol.Packet) (*Player, error) {
	r := bytes.NewReader(pkt.Data)
	username, err := protocol.ReadString(r)
	if err != nil {
		return nil, err
	}

	log.Printf("Player %s is logging in", username)

	// Generate offline-mode UUID (UUID v3 based on "OfflinePlayer:" + username)
	uuid := offlineUUID(username)

	s.mu.Lock()
	eid := s.nextEID
	s.nextEID++
	s.mu.Unlock()

	// Find spawn height at (8, 8) by checking the generator
	spawnY := float64(s.world.Gen.SurfaceHeight(8, 8)) + 1.0

	player := &Player{
		EntityID: eid,
		Username: username,
		UUID:     uuid,
		Conn:     conn,
		State:    protocol.StatePlay,
		GameMode: s.config.DefaultGameMode,
		X:        8,
		Y:        spawnY,
		Z:        8,
		Yaw:      0,
		Pitch:    0,
		OnGround: true,
		Health:   20.0,
		IsDead:   false,
		NoClip:   s.config.DefaultGameMode == GameModeSpectator,
	}

	// Initialize all inventory slots as empty
	for i := range player.Inventory {
		player.Inventory[i].ItemID = -1
	}
	player.Cursor.ItemID = -1
	player.ActiveSlot = 0
	player.CraftTableOutput.ItemID = -1
	for i := range player.CraftTableGrid {
		player.CraftTableGrid[i].ItemID = -1
	}

	// Send Login Success
	loginSuccess := protocol.MarshalPacket(0x02, func(w *bytes.Buffer) {
		protocol.WriteString(w, formatUUID(uuid))
		protocol.WriteString(w, username)
	})
	if err := protocol.WritePacket(conn, loginSuccess); err != nil {
		return nil, err
	}

	return player, nil
}

func (s *Server) handlePlay(player *Player) {
	conn := player.Conn

	// Send Join Game
	joinGame := protocol.MarshalPacket(0x01, func(w *bytes.Buffer) {
		protocol.WriteInt32(w, player.EntityID)          // Entity ID
		protocol.WriteByte(w, player.GameMode)           // Gamemode
		protocol.WriteByte(w, 0)                         // Dimension: overworld
		protocol.WriteByte(w, 0)                         // Difficulty: peaceful
		protocol.WriteByte(w, byte(s.config.MaxPlayers)) // Max players
		protocol.WriteString(w, "default")               // Level type
		protocol.WriteBool(w, false)                     // Reduced debug info
	})
	protocol.WritePacket(conn, joinGame)

	// Send Spawn Position
	spawnPos := protocol.MarshalPacket(0x05, func(w *bytes.Buffer) {
		protocol.WritePosition(w, 8, int32(player.Y), 8)
	})
	protocol.WritePacket(conn, spawnPos)

	// Send Player Abilities
	s.sendPlayerAbilities(player)

	// Send Player Position and Look
	posLook := protocol.MarshalPacket(0x08, func(w *bytes.Buffer) {
		protocol.WriteFloat64(w, player.X)
		protocol.WriteFloat64(w, player.Y)
		protocol.WriteFloat64(w, player.Z)
		protocol.WriteFloat32(w, player.Yaw)
		protocol.WriteFloat32(w, player.Pitch)
		protocol.WriteByte(w, 0) // Flags (all absolute)
	})
	protocol.WritePacket(conn, posLook)

	// Send chunks around player
	s.sendSpawnChunks(player)

	// Register player
	s.mu.Lock()
	s.players[player.EntityID] = player
	s.mu.Unlock()

	// Broadcast join message
	s.broadcastChat(chat.Colored(player.Username+" joined the game", "yellow"))

	log.Printf("Player %s (EID: %d) joined the game", player.Username, player.EntityID)

	// Start keep-alive ticker
	stopKeepAlive := make(chan struct{})
	go s.keepAliveLoop(player, stopKeepAlive)

	// Start health regeneration loop
	stopRegen := make(chan struct{})
	go s.regenerationLoop(player, stopRegen)

	// Start item pickup loop
	stopPickup := make(chan struct{})
	go s.itemPickupLoop(player, stopPickup)

	// Start environment loop
	stopEnv := make(chan struct{})
	go s.environmentLoop(player, stopEnv)

	defer func() {
		close(stopKeepAlive)
		close(stopRegen)
		close(stopPickup)
		close(stopEnv)
		// Remove from tab list before removing from players map
		s.broadcastPlayerListRemove(player.UUID)
		s.mu.Lock()
		delete(s.players, player.EntityID)
		s.mu.Unlock()
		s.broadcastChat(chat.Colored(player.Username+" left the game", "yellow"))
		// Despawn for other players
		s.broadcastDestroyEntity(player.EntityID)
		log.Printf("Player %s disconnected", player.Username)
	}()

	// Spawn this player for others and others for this player
	s.spawnPlayerForOthers(player)
	s.spawnOthersForPlayer(player)

	// Add self to own tab list so the player can see themselves
	selfListAdd := protocol.MarshalPacket(0x38, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, 0) // Action: Add Player
		protocol.WriteVarInt(w, 1) // Number of players
		protocol.WriteUUID(w, player.UUID)
		protocol.WriteString(w, player.Username)
		protocol.WriteVarInt(w, 0)                      // Number of properties
		protocol.WriteVarInt(w, int32(player.GameMode)) // Gamemode
		protocol.WriteVarInt(w, 0)                      // Ping
		protocol.WriteBool(w, false)                    // Has display name
	})
	player.mu.Lock()
	protocol.WritePacket(player.Conn, selfListAdd)
	player.mu.Unlock()

	// Spawn existing item entities for this player
	s.spawnEntitiesForPlayer(player)

	// Spawn existing mob entities for this player
	s.spawnMobEntitiesForPlayer(player)

	// Main packet read loop
	for {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		pkt, err := protocol.ReadPacket(conn)
		if err != nil {
			return
		}

		s.handlePlayPacket(player, pkt)
	}
}

func (s *Server) keepAliveLoop(player *Player, stop chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			keepAliveID := rand.Int31()
			pkt := protocol.MarshalPacket(0x00, func(w *bytes.Buffer) {
				protocol.WriteVarInt(w, keepAliveID)
			})
			player.mu.Lock()
			err := protocol.WritePacket(player.Conn, pkt)
			player.mu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

func (s *Server) regenerationLoop(player *Player, stop chan struct{}) {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			player.mu.Lock()
			if player.Health < 20.0 && !player.IsDead && player.GameMode == GameModeSurvival {
				player.Health += 1.0
				if player.Health > 20.0 {
					player.Health = 20.0
				}
				player.mu.Unlock()
				s.sendHealth(player)
			} else {
				player.mu.Unlock()
			}
		}
	}
}

func (s *Server) environmentLoop(player *Player, stop chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			// Fast check without locking player
			if !s.GameRuleBool("acidWater") {
				continue
			}

			player.mu.Lock()
			if player.IsDead || player.GameMode == GameModeCreative || player.GameMode == GameModeSpectator {
				player.mu.Unlock()
				continue
			}

			x, y, z := player.X, player.Y, player.Z
			player.mu.Unlock()

			bx := int32(math.Floor(x))
			byFeet := int32(math.Floor(y))
			byHead := int32(math.Floor(y + 1))
			bz := int32(math.Floor(z))

			blockFeet := s.world.GetBlock(bx, byFeet, bz) >> 4
			blockHead := s.world.GetBlock(bx, byHead, bz) >> 4

			if blockFeet == 8 || blockFeet == 9 || blockHead == 8 || blockHead == 9 {
				damage := s.GameRuleFloat("acidWaterDamage")
				if damage > 0 {
					s.applyDamage(player, damage, "melted in acid water")
				}
			}
		}
	}
}

func offlineUUID(username string) [16]byte {
	// Simple offline UUID generation (MD5 of "OfflinePlayer:" + username)
	// For simplicity, we'll use a deterministic hash
	data := []byte("OfflinePlayer:" + username)
	var uuid [16]byte

	// Simple hash
	h := uint64(0)
	for i, b := range data {
		h ^= uint64(b) << (uint(i%8) * 8)
	}
	uuid[0] = byte(h)
	uuid[1] = byte(h >> 8)
	uuid[2] = byte(h >> 16)
	uuid[3] = byte(h >> 24)
	uuid[4] = byte(h >> 32)
	uuid[5] = byte(h >> 40)
	uuid[6] = byte(h >> 48)
	uuid[7] = byte(h >> 56)

	// Second pass
	h2 := uint64(0x5555555555555555)
	for i, b := range data {
		h2 = h2*31 + uint64(b) + uint64(i)
	}
	uuid[8] = byte(h2)
	uuid[9] = byte(h2 >> 8)
	uuid[10] = byte(h2 >> 16)
	uuid[11] = byte(h2 >> 24)
	uuid[12] = byte(h2 >> 32)
	uuid[13] = byte(h2 >> 40)
	uuid[14] = byte(h2 >> 48)
	uuid[15] = byte(h2 >> 56)

	// Set version to 3 (name-based)
	uuid[6] = (uuid[6] & 0x0F) | 0x30
	// Set variant to RFC 4122
	uuid[8] = (uuid[8] & 0x3F) | 0x80

	return uuid
}

func formatUUID(uuid [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}
