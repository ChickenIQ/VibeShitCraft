package server

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"github.com/StoreStation/VibeShitCraft/pkg/protocol"
)

// Gamemode constants matching Minecraft protocol values.
const (
	GameModeSurvival  byte = 0
	GameModeCreative  byte = 1
	GameModeAdventure byte = 2
	GameModeSpectator byte = 3
)

// Entity metadata flags (index 0, type byte).
const (
	EntityFlagInvisible byte = 0x20
)

// ParseGameMode parses a gamemode string into its byte value.
// Returns the mode and true on success, or 0 and false on failure.
func ParseGameMode(s string) (byte, bool) {
	switch strings.ToLower(s) {
	case "survival", "s", "0":
		return GameModeSurvival, true
	case "creative", "c", "1":
		return GameModeCreative, true
	case "adventure", "a", "2":
		return GameModeAdventure, true
	case "spectator", "sp", "3":
		return GameModeSpectator, true
	default:
		return 0, false
	}
}

// GameModeName returns the display name for a gamemode.
func GameModeName(mode byte) string {
	switch mode {
	case GameModeSurvival:
		return "Survival"
	case GameModeCreative:
		return "Creative"
	case GameModeAdventure:
		return "Adventure"
	case GameModeSpectator:
		return "Spectator"
	default:
		return fmt.Sprintf("Unknown(%d)", mode)
	}
}

// switchGameMode changes a player's gamemode, sending all necessary packets
// to the player and broadcasting updates to other players.
func (s *Server) switchGameMode(player *Player, mode byte) {
	player.mu.Lock()
	player.GameMode = mode
	player.NoClip = mode == GameModeSpectator
	player.mu.Unlock()

	// Send Change Game State packet (reason=3 = change game mode)
	changeGameState := protocol.MarshalPacket(0x2B, func(w *bytes.Buffer) {
		protocol.WriteByte(w, 3)                // Reason: change game mode
		protocol.WriteFloat32(w, float32(mode)) // Value: new game mode
	})
	player.mu.Lock()
	protocol.WritePacket(player.Conn, changeGameState)
	player.mu.Unlock()

	// Send updated abilities
	s.sendPlayerAbilities(player)

	// Update gamemode in player list for all clients
	s.broadcastPlayerListGamemode(player)

	// Update entity visibility (spectators are invisible)
	s.broadcastEntityFlags(player)

	modeName := GameModeName(mode)
	log.Printf("Player %s game mode changed to %s", player.Username, modeName)
}

// sendPlayerAbilities sends the Player Abilities packet (0x39) based on the player's current gamemode.
func (s *Server) sendPlayerAbilities(player *Player) {
	var flags byte
	switch player.GameMode {
	case GameModeCreative:
		flags = 0x0D // Invulnerable (0x01) | Allow Flying (0x04) | Instant Break (0x08)
	case GameModeSpectator:
		flags = 0x07 // Invulnerable (0x01) | Flying (0x02) | Allow Flying (0x04)
	default:
		flags = 0x00
	}
	abilities := protocol.MarshalPacket(0x39, func(w *bytes.Buffer) {
		protocol.WriteByte(w, flags)
		protocol.WriteFloat32(w, 0.05) // Flying speed
		protocol.WriteFloat32(w, 0.1)  // Walking speed (FOV modifier)
	})
	player.mu.Lock()
	protocol.WritePacket(player.Conn, abilities)
	player.mu.Unlock()
}

// broadcastPlayerListGamemode sends a Player List Item (action=1, Update Gamemode)
// to all players, updating the target player's gamemode in the tab list.
func (s *Server) broadcastPlayerListGamemode(player *Player) {
	player.mu.Lock()
	gameMode := player.GameMode
	uuid := player.UUID
	player.mu.Unlock()

	pkt := protocol.MarshalPacket(0x38, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, 1) // Action: Update Gamemode
		protocol.WriteVarInt(w, 1) // Number of players
		protocol.WriteUUID(w, uuid)
		protocol.WriteVarInt(w, int32(gameMode)) // New gamemode
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

// broadcastEntityFlags sends an Entity Metadata packet (0x1C) to all players
// with updated entity flags (index 0) for the given player.
// In spectator mode, the invisible flag (0x20) is set so the player appears
// as a transparent head to other spectators and is invisible to non-spectators.
func (s *Server) broadcastEntityFlags(player *Player) {
	player.mu.Lock()
	var flags byte
	if player.GameMode == GameModeSpectator {
		flags = EntityFlagInvisible
	}
	entityID := player.EntityID
	player.mu.Unlock()

	pkt := protocol.MarshalPacket(0x1C, func(w *bytes.Buffer) {
		protocol.WriteVarInt(w, entityID)
		protocol.WriteByte(w, 0x00) // header: (type 0 << 5) | index 0 = entity flags
		protocol.WriteByte(w, flags)
		protocol.WriteByte(w, 0x7F) // Metadata terminator
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
