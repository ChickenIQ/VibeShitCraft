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

	"github.com/StoreStation/VibeShitCraft/pkg/protocol"
	"github.com/StoreStation/VibeShitCraft/pkg/world"
)

const DefaultSeed = 0

// Config holds the server configuration.
type Config struct {
	Address         string
	MaxPlayers      int
	MOTD            string
	Seed            int64
	DefaultGameMode byte
}

// DefaultConfig returns a default server configuration.
func DefaultConfig() Config {
	return Config{
		Address:    ":25565",
		MaxPlayers: 20,
		MOTD:       "A VibeShitCraft Server",
	}
}

// ViewDistance is the number of chunks sent around a player in each direction.
const ViewDistance = 7

// ChunkPos identifies a chunk by its X and Z coordinates.
type ChunkPos struct {
	X, Z int32
}

// Server is the core Minecraft server managing players, entities, and the world.
type Server struct {
	config      Config
	listener    net.Listener
	mu          sync.RWMutex
	players     map[int32]*Player
	entities    map[int32]*ItemEntity
	mobEntities map[int32]*MobEntity
	nextEID     int32
	stopCh      chan struct{}
	stopOnce    sync.Once
	world       *world.World
	gamerules   map[string]string
}

// New creates a new server with the given configuration.
func New(config Config) *Server {
	seed := config.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	log.Printf("World seed: %d", seed)
	return &Server{
		config:      config,
		players:     make(map[int32]*Player),
		entities:    make(map[int32]*ItemEntity),
		mobEntities: make(map[int32]*MobEntity),
		nextEID:     1,
		stopCh:      make(chan struct{}),
		world:       world.NewWorld(seed),
		gamerules: map[string]string{
			"acidWater":       "false",
			"acidWaterDamage": "1.0",
		},
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
	go s.entityPhysicsLoop()
	go s.randomTickLoop()
	return nil
}

// randomTickLoop simulates random block updates (e.g. crop growth).
func (s *Server) randomTickLoop() {
	ticker := time.NewTicker(2 * time.Second) // Tick every 2 seconds
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			// Get all currently loaded chunk positions (simple approach: gather from all players)
			loadedChunks := make(map[ChunkPos]struct{})
			s.mu.RLock()
			for _, p := range s.players {
				p.mu.Lock()
				px := int32(p.X) >> 4
				pz := int32(p.Z) >> 4
				p.mu.Unlock()
				for dx := -int32(ViewDistance); dx <= int32(ViewDistance); dx++ {
					for dz := -int32(ViewDistance); dz <= int32(ViewDistance); dz++ {
						loadedChunks[ChunkPos{X: px + dx, Z: pz + dz}] = struct{}{}
					}
				}
			}
			s.mu.RUnlock()

			// Perform random ticks in each section
			for pos := range loadedChunks {
				startX := pos.X << 4
				startZ := pos.Z << 4

				for y := int32(0); y < 256; y += 16 {
					// Vanilla does 3 random blocks per section (16x16x16) per chunk tick.
					for i := 0; i < 3; i++ {
						rx := startX + int32(rand.Intn(16))
						ry := y + int32(rand.Intn(16))
						rz := startZ + int32(rand.Intn(16))

						blockState := s.world.GetBlock(rx, ry, rz)
						blockID := blockState >> 4
						metadata := blockState & 0x0F

						switch blockID {
						case 59, 141, 142: // Crops (Wheat, Carrots, Potatoes)
							belowBlockID := s.world.GetBlock(rx, ry-1, rz) >> 4
							if belowBlockID == 60 {
								if metadata < 7 && rand.Float32() < 0.25 {
									newState := (blockID << 4) | (metadata + 1)
									s.world.SetBlock(rx, ry, rz, newState)
									s.broadcastBlockChange(rx, ry, rz, newState)
								}
							} else {
								// Break the crop if farmland is gone
								s.world.SetBlock(rx, ry, rz, 0)
								s.broadcastBlockChange(rx, ry, rz, 0)
								itemID, damage, count := world.BlockToItemID(blockState)
								if itemID > 0 {
									s.SpawnItem(float64(rx)+0.5, float64(ry)+0.5, float64(rz)+0.5, 0, 0.2, 0, itemID, damage, count)
								}
							}
						case 104, 105: // Stems (Pumpkin, Melon)
							belowBlockID := s.world.GetBlock(rx, ry-1, rz) >> 4
							if belowBlockID != 60 {
								s.world.SetBlock(rx, ry, rz, 0)
								s.broadcastBlockChange(rx, ry, rz, 0)
								itemID, damage, count := world.BlockToItemID(blockState)
								if itemID > 0 {
									s.SpawnItem(float64(rx)+0.5, float64(ry)+0.5, float64(rz)+0.5, 0, 0.2, 0, itemID, damage, count)
								}
							} else if metadata < 7 {
								if rand.Float32() < 0.25 {
									newState := (blockID << 4) | (metadata + 1)
									s.world.SetBlock(rx, ry, rz, newState)
									s.broadcastBlockChange(rx, ry, rz, newState)
								}
							} else if metadata == 7 {
								if rand.Float32() < 0.25 {
									// Try to spawn fruit
									adj := []struct{ dx, dz int32 }{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
									dir := adj[rand.Intn(4)]
									fx, fz := rx+dir.dx, rz+dir.dz
									adjBlock := s.world.GetBlock(fx, ry, fz) >> 4
									belowAdjBlock := s.world.GetBlock(fx, ry-1, fz) >> 4
									if adjBlock == 0 && (belowAdjBlock == 2 || belowAdjBlock == 3 || belowAdjBlock == 60) {
										fruitID := uint16(86) // Pumpkin
										if blockID == 105 {
											fruitID = 103 // Melon
										}
										newFruitState := fruitID << 4
										s.world.SetBlock(fx, ry, fz, newFruitState)
										s.broadcastBlockChange(fx, ry, fz, newFruitState)
									}
								}
							}
						case 83, 81: // Sugar Cane, Cactus
							// Check validity
							valid := true
							belowID := s.world.GetBlock(rx, ry-1, rz) >> 4
							if blockID == 83 && belowID != 83 && belowID != 12 && belowID != 3 && belowID != 2 {
								valid = false
							} else if blockID == 81 {
								if belowID != 81 && belowID != 12 {
									valid = false
								} else {
									// Cactus cannot be adjacent to a solid block
									adj := []struct{ dx, dz int32 }{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
									for _, d := range adj {
										adjBlock := s.world.GetBlock(rx+d.dx, ry, rz+d.dz) >> 4
										if adjBlock != 0 && adjBlock != 8 && adjBlock != 9 && adjBlock != 10 && adjBlock != 11 {
											valid = false
											break
										}
									}
								}
							}
							if !valid {
								s.world.SetBlock(rx, ry, rz, 0)
								s.broadcastBlockChange(rx, ry, rz, 0)
								s.SpawnItem(float64(rx)+0.5, float64(ry)+0.5, float64(rz)+0.5, 0, 0.2, 0, int16(blockID), 0, 1)
							} else {
								aboveBlock := s.world.GetBlock(rx, ry+1, rz) >> 4
								if aboveBlock == 0 {
									if metadata < 15 {
										if rand.Float32() < 0.25 {
											newState := (blockID << 4) | (metadata + 1)
											s.world.SetBlock(rx, ry, rz, newState)
											s.broadcastBlockChange(rx, ry, rz, newState)
										}
									} else {
										// Check height
										var h int32 = 1
										for currY := ry - 1; currY > ry-3; currY-- {
											if s.world.GetBlock(rx, currY, rz)>>4 == blockID {
												h++
											} else {
												break
											}
										}
										if h < 3 {
											// Grow up
											newState := blockID << 4 // new block with meta 0
											s.world.SetBlock(rx, ry+1, rz, newState)
											s.broadcastBlockChange(rx, ry+1, rz, newState)
											// Reset this block's meta to 0
											s.world.SetBlock(rx, ry, rz, blockID<<4)
											s.broadcastBlockChange(rx, ry, rz, blockID<<4)
										}
									}
								}
							}
						case 6: // Sapling
							if rand.Float32() < 0.10 {
								woodType := metadata & 0x07
								size := 1
								tx, tz := rx, rz

								if woodType == 5 { // Dark Oak requires 3x3 for giant
									if nx, nz, found := s.check3x3Sapling(rx, ry, rz, uint16(metadata)); found {
										size = 3
										tx, tz = nx, nz
									} else {
										// Skip growth
										size = 0
									}
								} else {
									if nx, nz, found := s.check2x2Sapling(rx, ry, rz, uint16(metadata)); found {
										size = 2
										tx, tz = nx, nz
									}
								}

								if size > 0 {
									s.growTree(tx, ry, tz, uint16(metadata), size)
								}
							}
						}
					}
				}
			}
		}
	}
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
		if s.listener != nil {
			s.listener.Close()
		}
		s.mu.RLock()
		for _, p := range s.players {
			if p.Conn != nil {
				p.Conn.Close()
			}
		}
		s.mu.RUnlock()
	})
}

// StopChan returns a channel that is closed when the server is stopped.
func (s *Server) StopChan() <-chan struct{} {
	return s.stopCh
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

func (s *Server) playerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.players)
}

func (s *Server) GameRuleBool(name string) bool {
	s.mu.RLock()
	val, ok := s.gamerules[name]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	return val == "true"
}

func (s *Server) GameRuleFloat(name string) float32 {
	s.mu.RLock()
	val, ok := s.gamerules[name]
	s.mu.RUnlock()
	if !ok {
		return 0.0
	}
	var fval float32
	fmt.Sscanf(val, "%f", &fval)
	return fval
}
