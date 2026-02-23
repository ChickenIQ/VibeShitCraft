package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
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
	world       *world.World
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

func (s *Server) playerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.players)
}
