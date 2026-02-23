package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/StoreStation/VibeShitCraft/pkg/chat"
	"github.com/StoreStation/VibeShitCraft/pkg/protocol"
	"github.com/StoreStation/VibeShitCraft/pkg/world"
)

// Config holds server configuration.
type Config struct {
	Address    string
	MaxPlayers int
	MOTD       string
}

// DefaultConfig returns a default server configuration.
func DefaultConfig() Config {
	return Config{
		Address:    ":25565",
		MaxPlayers: 20,
		MOTD:       "A VibeShitCraft Server",
	}
}

// Server represents a Minecraft 1.8 server.
type Server struct {
	config   Config
	listener net.Listener
	players  map[int32]*Player
	mu       sync.RWMutex
	nextEID  int32
	stopCh   chan struct{}
	world    *world.World
}

// Slot represents an inventory slot.
type Slot struct {
	ItemID int16
	Count  byte
	Damage int16
}

// Player represents a connected player.
type Player struct {
	EntityID  int32
	Username  string
	UUID      [16]byte
	Conn      net.Conn
	State     int
	X, Y, Z   float64
	Yaw       float32
	Pitch     float32
	OnGround  bool
	Inventory [45]Slot
	mu        sync.Mutex
}

// New creates a new server with the given configuration.
func New(config Config) *Server {
	return &Server{
		config:  config,
		players: make(map[int32]*Player),
		nextEID: 1,
		stopCh:  make(chan struct{}),
		world:   world.NewWorld(),
	}
}

// Start begins listening for connections.
func (s *Server) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", s.config.Address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.Address, err)
	}
	log.Printf("Server listening on %s", s.config.Address)

	go s.acceptLoop()
	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() {
	close(s.stopCh)
	if s.listener != nil {
		s.listener.Close()
	}
	s.mu.RLock()
	for _, p := range s.players {
		p.Conn.Close()
	}
	s.mu.RUnlock()
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				log.Printf("Accept error: %v", err)
				continue
			}
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	state := protocol.StateHandshaking

	for {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		pkt, err := protocol.ReadPacket(conn)
		if err != nil {
			return
		}

		switch state {
		case protocol.StateHandshaking:
			if pkt.ID == 0x00 {
				newState, err := s.handleHandshake(pkt)
				if err != nil {
					log.Printf("Handshake error: %v", err)
					return
				}
				state = newState
			}
		case protocol.StateStatus:
			switch pkt.ID {
			case 0x00:
				s.handleStatusRequest(conn)
			case 0x01:
				s.handlePing(conn, pkt)
				return
			}
		case protocol.StateLogin:
			if pkt.ID == 0x00 {
				player, err := s.handleLoginStart(conn, pkt)
				if err != nil {
					log.Printf("Login error: %v", err)
					return
				}
				s.handlePlay(player)
				return
			}
		}
	}
}

func (s *Server) handleHandshake(pkt *protocol.Packet) (int, error) {
	r := bytes.NewReader(pkt.Data)

	protocolVersion, _, err := protocol.ReadVarInt(r)
	if err != nil {
		return 0, err
	}
	_ = protocolVersion

	// Server address
	_, err = protocol.ReadString(r)
	if err != nil {
		return 0, err
	}

	// Server port
	_, err = protocol.ReadUint16(r)
	if err != nil {
		return 0, err
	}

	// Next state
	nextState, _, err := protocol.ReadVarInt(r)
	if err != nil {
		return 0, err
	}

	return int(nextState), nil
}

func (s *Server) handleStatusRequest(conn net.Conn) {
	response := map[string]interface{}{
		"version": map[string]interface{}{
			"name":     "1.8.9",
			"protocol": protocol.ProtocolVersion,
		},
		"players": map[string]interface{}{
			"max":    s.config.MaxPlayers,
			"online": s.playerCount(),
			"sample": []interface{}{},
		},
		"description": map[string]interface{}{
			"text": s.config.MOTD,
		},
	}

	jsonResp, err := json.Marshal(response)
	if err != nil {
		log.Printf("Failed to marshal status response: %v", err)
		return
	}
	pkt := protocol.MarshalPacket(0x00, func(w *bytes.Buffer) {
		protocol.WriteString(w, string(jsonResp))
	})
	protocol.WritePacket(conn, pkt)
}

func (s *Server) handlePing(conn net.Conn, pkt *protocol.Packet) {
	r := bytes.NewReader(pkt.Data)
	payload, err := protocol.ReadInt64(r)
	if err != nil {
		return
	}

	resp := protocol.MarshalPacket(0x01, func(w *bytes.Buffer) {
		protocol.WriteInt64(w, payload)
	})
	protocol.WritePacket(conn, resp)
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

	player := &Player{
		EntityID: eid,
		Username: username,
		UUID:     uuid,
		Conn:     conn,
		State:    protocol.StatePlay,
		X:        8,
		Y:        5, // Above the flat world surface
		Z:        8,
		Yaw:      0,
		Pitch:    0,
		OnGround: true,
	}

	// Initialize all inventory slots as empty
	for i := range player.Inventory {
		player.Inventory[i].ItemID = -1
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
		protocol.WriteInt32(w, player.EntityID)            // Entity ID
		protocol.WriteByte(w, 0)                           // Gamemode: survival
		protocol.WriteByte(w, 0)                           // Dimension: overworld
		protocol.WriteByte(w, 0)                           // Difficulty: peaceful
		protocol.WriteByte(w, byte(s.config.MaxPlayers))   // Max players
		protocol.WriteString(w, "flat")                    // Level type
		protocol.WriteBool(w, false)                       // Reduced debug info
	})
	protocol.WritePacket(conn, joinGame)

	// Send Spawn Position
	spawnPos := protocol.MarshalPacket(0x05, func(w *bytes.Buffer) {
		protocol.WritePosition(w, 8, 5, 8)
	})
	protocol.WritePacket(conn, spawnPos)

	// Send Player Abilities
	abilities := protocol.MarshalPacket(0x39, func(w *bytes.Buffer) {
		protocol.WriteByte(w, 0x00)       // Flags (none)
		protocol.WriteFloat32(w, 0.05)    // Flying speed
		protocol.WriteFloat32(w, 0.1)     // Walking speed (FOV modifier)
	})
	protocol.WritePacket(conn, abilities)

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

	// Send chunks around spawn
	s.sendSpawnChunks(conn)

	// Send any block modifications to the new player
	s.sendBlockModifications(conn)

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

	defer func() {
		close(stopKeepAlive)
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

func (s *Server) handlePlayPacket(player *Player, pkt *protocol.Packet) {
	r := bytes.NewReader(pkt.Data)

	switch pkt.ID {
	case 0x00: // Keep Alive
		// Client responding to keep alive, ignore

	case 0x01: // Chat Message
		message, err := protocol.ReadString(r)
		if err != nil {
			return
		}
		if len(message) > 256 {
			message = message[:256]
		}
		chatMsg := chat.Message{
			Text: "",
			Extra: []chat.Message{
				chat.Colored("<"+player.Username+"> ", "white"),
				chat.Text(message),
			},
		}
		log.Printf("<%s> %s", player.Username, message)
		s.broadcastChat(chatMsg)

	case 0x04: // Player Position
		x, _ := protocol.ReadFloat64(r)
		y, _ := protocol.ReadFloat64(r)
		z, _ := protocol.ReadFloat64(r)
		onGround, _ := protocol.ReadBool(r)
		player.mu.Lock()
		player.X = x
		player.Y = y
		player.Z = z
		player.OnGround = onGround
		player.mu.Unlock()
		s.broadcastEntityTeleport(player)

	case 0x05: // Player Look
		yaw, _ := protocol.ReadFloat32(r)
		pitch, _ := protocol.ReadFloat32(r)
		onGround, _ := protocol.ReadBool(r)
		player.mu.Lock()
		player.Yaw = yaw
		player.Pitch = pitch
		player.OnGround = onGround
		player.mu.Unlock()
		s.broadcastEntityLook(player)

	case 0x06: // Player Position And Look
		x, _ := protocol.ReadFloat64(r)
		y, _ := protocol.ReadFloat64(r)
		z, _ := protocol.ReadFloat64(r)
		yaw, _ := protocol.ReadFloat32(r)
		pitch, _ := protocol.ReadFloat32(r)
		onGround, _ := protocol.ReadBool(r)
		player.mu.Lock()
		player.X = x
		player.Y = y
		player.Z = z
		player.Yaw = yaw
		player.Pitch = pitch
		player.OnGround = onGround
		player.mu.Unlock()
		s.broadcastEntityTeleport(player)

	case 0x03: // Player (on ground)
		// Just an update for on ground status
		onGround, _ := protocol.ReadBool(r)
		player.mu.Lock()
		player.OnGround = onGround
		player.mu.Unlock()

	case 0x09: // Held Item Change
		// Ignore for now

	case 0x07: // Player Digging
		status, _ := protocol.ReadByte(r)
		x, y, z, _ := protocol.ReadPosition(r)
		_, _ = protocol.ReadByte(r) // face
		if status == 2 { // Finished digging
			s.handleBlockBreak(player, x, y, z)
		}

	case 0x0A: // Animation
		// Broadcast arm swing to other players
		s.broadcastAnimation(player, 0)

	case 0x15: // Client Settings
		// Ignore for now

	case 0x17: // Plugin Message
		// Ignore for now

	case 0x16: // Client Status
		// Ignore for now
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

func (s *Server) sendSpawnChunks(conn net.Conn) {
	chunkData, primaryBitMask := world.GenerateFlatChunkData()

	// Send a 7x7 grid of chunks around spawn
	for cx := -3; cx <= 3; cx++ {
		for cz := -3; cz <= 3; cz++ {
			pkt := protocol.MarshalPacket(0x21, func(w *bytes.Buffer) {
				protocol.WriteInt32(w, int32(cx))         // Chunk X
				protocol.WriteInt32(w, int32(cz))         // Chunk Z
				protocol.WriteBool(w, true)               // Ground-up continuous
				protocol.WriteUint16(w, primaryBitMask)   // Primary bit mask
				protocol.WriteVarInt(w, int32(len(chunkData))) // Size
				w.Write(chunkData)                         // Data
			})
			protocol.WritePacket(conn, pkt)
		}
	}
}

func (s *Server) sendBlockModifications(conn net.Conn) {
	modifications := s.world.GetModifications()
	for pos, blockState := range modifications {
		pkt := protocol.MarshalPacket(0x23, func(w *bytes.Buffer) {
			protocol.WritePosition(w, pos.X, pos.Y, pos.Z)
			protocol.WriteVarInt(w, int32(blockState))
		})
		protocol.WritePacket(conn, pkt)
	}
}

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
		protocol.WritePacket(p.Conn, pkt)
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
	target.mu.Unlock()

	// Player List Item (Add Player) - 0x38
	playerListAdd := protocol.MarshalPacket(0x38, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, 0) // Action: Add Player
		protocol.WriteVarInt(w, 1) // Number of players
		protocol.WriteUUID(w, target.UUID)
		protocol.WriteString(w, target.Username)
		protocol.WriteVarInt(w, 0) // Number of properties
		protocol.WriteVarInt(w, 0) // Gamemode: survival
		protocol.WriteVarInt(w, 0) // Ping
		protocol.WriteBool(w, false) // Has display name
	})
	viewer.mu.Lock()
	protocol.WritePacket(viewer.Conn, playerListAdd)
	viewer.mu.Unlock()

	// Spawn Player - 0x0C
	spawnPlayer := protocol.MarshalPacket(0x0C, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, target.EntityID)
		protocol.WriteUUID(w, target.UUID)
		protocol.WriteInt32(w, int32(x*32))    // Fixed-point X
		protocol.WriteInt32(w, int32(y*32))    // Fixed-point Y
		protocol.WriteInt32(w, int32(z*32))    // Fixed-point Z
		protocol.WriteByte(w, byte(yaw*256/360))
		protocol.WriteByte(w, byte(pitch*256/360))
		protocol.WriteInt16(w, 0)              // Current item
		protocol.WriteByte(w, 0x7F)            // Metadata terminator
	})
	viewer.mu.Lock()
	protocol.WritePacket(viewer.Conn, spawnPlayer)
	viewer.mu.Unlock()
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
		protocol.WritePacket(p.Conn, pkt)
		p.mu.Unlock()
	}
}

func (s *Server) handleBlockBreak(player *Player, x, y, z int32) {
	blockState := s.world.GetBlock(x, y, z)
	blockID := blockState >> 4

	// Don't break air or bedrock
	if blockID == 0 || blockID == 7 {
		return
	}

	// Set block to air in world state
	s.world.SetBlock(x, y, z, 0)

	// Broadcast block change (air) to all players
	s.broadcastBlockChange(x, y, z, 0)

	// Broadcast block break effect (particles/sound) to other players
	s.broadcastBlockBreakEffect(player, x, y, z, blockState)

	// Give item to player
	itemID := world.BlockToItemID(blockState)
	if itemID < 0 {
		return
	}

	player.mu.Lock()
	slotIndex, ok := addItemToInventory(player, itemID, 1)
	if ok {
		slot := player.Inventory[slotIndex]
		pkt := protocol.MarshalPacket(0x2F, func(w *bytes.Buffer) {
			protocol.WriteByte(w, 0) // Window ID 0 = player inventory
			protocol.WriteInt16(w, int16(slotIndex))
			protocol.WriteSlotData(w, slot.ItemID, slot.Count, slot.Damage)
		})
		protocol.WritePacket(player.Conn, pkt)
	}
	player.mu.Unlock()

	if ok {
		log.Printf("Player %s broke block %d at (%d, %d, %d), received item %d", player.Username, blockID, x, y, z, itemID)
	}
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
		protocol.WritePacket(p.Conn, pkt)
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
		protocol.WritePacket(p.Conn, pkt)
		p.mu.Unlock()
	}
}

// addItemToInventory finds a suitable slot and adds the item to the player's inventory.
// Returns the slot index and true if successful, or -1 and false if inventory is full.
// Must be called with player.mu held.
func addItemToInventory(player *Player, itemID int16, count byte) (int, bool) {
	// Try to stack in hotbar (slots 36-44)
	for i := 36; i <= 44; i++ {
		if player.Inventory[i].ItemID == itemID && player.Inventory[i].Count+count <= 64 {
			player.Inventory[i].Count += count
			return i, true
		}
	}
	// Try to stack in main inventory (slots 9-35)
	for i := 9; i <= 35; i++ {
		if player.Inventory[i].ItemID == itemID && player.Inventory[i].Count+count <= 64 {
			player.Inventory[i].Count += count
			return i, true
		}
	}
	// Try empty slot in hotbar
	for i := 36; i <= 44; i++ {
		if player.Inventory[i].ItemID == -1 {
			player.Inventory[i] = Slot{ItemID: itemID, Count: count}
			return i, true
		}
	}
	// Try empty slot in main inventory
	for i := 9; i <= 35; i++ {
		if player.Inventory[i].ItemID == -1 {
			player.Inventory[i] = Slot{ItemID: itemID, Count: count}
			return i, true
		}
	}
	return -1, false
}

func (s *Server) playerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.players)
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
