package server

import (
	"bytes"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/StoreStation/VibeShitCraft/pkg/chat"
	"github.com/StoreStation/VibeShitCraft/pkg/protocol"
)

// handleCommand dispatches a /-prefixed command from a player.
func (s *Server) handleCommand(player *Player, message string) {
	parts := strings.Fields(message)
	if len(parts) == 0 {
		return
	}
	cmd := strings.ToLower(parts[0])
	log.Printf("Player %s issued command: %s", player.Username, message)

	switch cmd {
	case "/gamemode", "/gm":
		s.handleGamemodeCommand(player, parts[1:])
	case "/tp", "/teleport":
		s.handleTpCommand(player, parts[1:])
	case "/stop":
		s.handleStopCommand(player)
	default:
		s.sendChatToPlayer(player, chat.Colored("Unknown command: "+cmd, "red"))
	}
}

// handleGamemodeCommand handles the /gamemode command.
// Usage: /gamemode <survival|creative|adventure|spectator|0|1|2|3>
func (s *Server) handleGamemodeCommand(player *Player, args []string) {
	if len(args) < 1 {
		s.sendChatToPlayer(player, chat.Colored("Usage: /gamemode <survival|creative|adventure|spectator|0|1|2|3>", "red"))
		return
	}

	var mode byte
	switch strings.ToLower(args[0]) {
	case "survival", "s", "0":
		mode = GameModeSurvival
	case "creative", "c", "1":
		mode = GameModeCreative
	case "adventure", "a", "2":
		mode = GameModeAdventure
	case "spectator", "sp", "3":
		mode = GameModeSpectator
	default:
		s.sendChatToPlayer(player, chat.Colored("Unknown gamemode: "+args[0], "red"))
		return
	}

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

	// Feedback
	modeName := GameModeName(mode)
	s.sendChatToPlayer(player, chat.Colored("Game mode set to "+modeName, "gray"))
	log.Printf("Player %s game mode changed to %s", player.Username, modeName)
}

// handleTpCommand handles the /tp command.
// Usage: /tp <x> <y> <z> — teleport to coordinates
// Usage: /tp <player>    — teleport to another player
func (s *Server) handleTpCommand(player *Player, args []string) {
	if len(args) == 3 {
		// /tp <x> <y> <z>
		x, err1 := strconv.ParseFloat(args[0], 64)
		y, err2 := strconv.ParseFloat(args[1], 64)
		z, err3 := strconv.ParseFloat(args[2], 64)
		if err1 != nil || err2 != nil || err3 != nil {
			s.sendChatToPlayer(player, chat.Colored("Invalid coordinates. Usage: /tp <x> <y> <z>", "red"))
			return
		}
		s.teleportPlayer(player, x, y, z)
		s.sendChatToPlayer(player, chat.Colored(fmt.Sprintf("Teleported to %.1f, %.1f, %.1f", x, y, z), "gray"))
		log.Printf("Player %s teleported to %.1f, %.1f, %.1f", player.Username, x, y, z)
	} else if len(args) == 1 {
		// /tp <player>
		targetName := args[0]
		s.mu.RLock()
		var target *Player
		for _, p := range s.players {
			if strings.EqualFold(p.Username, targetName) {
				target = p
				break
			}
		}
		s.mu.RUnlock()

		if target == nil {
			s.sendChatToPlayer(player, chat.Colored("Player not found: "+targetName, "red"))
			return
		}

		target.mu.Lock()
		tx, ty, tz := target.X, target.Y, target.Z
		target.mu.Unlock()

		s.teleportPlayer(player, tx, ty, tz)
		s.sendChatToPlayer(player, chat.Colored("Teleported to "+target.Username, "gray"))
		log.Printf("Player %s teleported to %s (%.1f, %.1f, %.1f)", player.Username, target.Username, tx, ty, tz)
	} else {
		s.sendChatToPlayer(player, chat.Colored("Usage: /tp <x> <y> <z> or /tp <player>", "red"))
	}
}

// teleportPlayer moves a player to the given coordinates and syncs the change.
func (s *Server) teleportPlayer(player *Player, x, y, z float64) {
	player.mu.Lock()
	player.X = x
	player.Y = y
	player.Z = z
	player.mu.Unlock()

	// Send Player Position And Look to the teleported player
	posLook := protocol.MarshalPacket(0x08, func(w *bytes.Buffer) {
		protocol.WriteFloat64(w, x)
		protocol.WriteFloat64(w, y)
		protocol.WriteFloat64(w, z)
		protocol.WriteFloat32(w, player.Yaw)
		protocol.WriteFloat32(w, player.Pitch)
		protocol.WriteByte(w, 0) // Flags (all absolute)
	})
	player.mu.Lock()
	protocol.WritePacket(player.Conn, posLook)
	player.mu.Unlock()

	// Broadcast teleport to other players
	s.broadcastEntityTeleport(player)

	// Load/unload chunks around new position
	s.sendChunkUpdates(player)
}

// handleStopCommand handles the /stop command.
func (s *Server) handleStopCommand(player *Player) {
	log.Printf("Player %s issued /stop command, shutting down server...", player.Username)
	s.broadcastChat(chat.Colored("Server is stopping...", "red"))

	// Give a small delay for the message to propagate if needed,
	// though Stop() closes connections which is more immediate.
	go func() {
		time.Sleep(500 * time.Millisecond)
		s.Stop()
	}()
}
