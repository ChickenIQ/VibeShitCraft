package server

import (
	"bytes"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/VibeShit/VibeShitCraft/pkg/chat"
	"github.com/VibeShit/VibeShitCraft/pkg/protocol"
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
	case "/gamerule":
		s.handleGameruleCommand(player, parts[1:])
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
	if player.Conn != nil {
		protocol.WritePacket(player.Conn, posLook)
	}
	player.mu.Unlock()

	// Broadcast teleport to other players
	s.broadcastEntityTeleport(player)

	// Load/unload chunks around new position
	s.sendChunkUpdates(player)

	// Update entity tracking at destination
	s.updateEntityTracking(player)
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

// handleGameruleCommand handles the /gamerule command.
func (s *Server) handleGameruleCommand(player *Player, args []string) {
	if len(args) == 0 {
		s.mu.RLock()
		var rules []string
		for k := range s.gamerules {
			rules = append(rules, k)
		}
		s.mu.RUnlock()
		s.sendChatToPlayer(player, chat.Colored("Gamerules: "+strings.Join(rules, ", "), "gray"))
		return
	}

	rule := args[0]
	if len(args) == 1 {
		s.mu.RLock()
		val, ok := s.gamerules[rule]
		s.mu.RUnlock()
		if !ok {
			s.sendChatToPlayer(player, chat.Colored("Unknown gamerule: "+rule, "red"))
		} else {
			s.sendChatToPlayer(player, chat.Colored(rule+" = "+val, "gray"))
		}
		return
	}

	val := args[1]

	s.mu.Lock()
	if _, ok := s.gamerules[rule]; !ok {
		s.mu.Unlock()
		s.sendChatToPlayer(player, chat.Colored("Unknown gamerule: "+rule, "red"))
		return
	}
	s.gamerules[rule] = val
	s.mu.Unlock()

	msg := fmt.Sprintf("Gamerule %s updated to %s", rule, val)
	log.Printf("Player %s: %s", player.Username, msg)
	s.broadcastChat(chat.Colored(msg, "gray"))
}

// handleTabComplete handles Serverbound packet 0x14 for command tab completion.
func (s *Server) handleTabComplete(player *Player, text string) {
	text = strings.TrimPrefix(text, "/")
	parts := strings.Split(text, " ")

	var matches []string

	if len(parts) == 1 {
		// Command name completion
		cmds := []string{"gamemode", "tp", "gamerule", "stop"}
		prefix := strings.ToLower(parts[0])
		for _, cmd := range cmds {
			if strings.HasPrefix(cmd, prefix) {
				matches = append(matches, "/"+cmd)
			}
		}
	} else if len(parts) > 1 {
		// Argument completion
		cmd := strings.ToLower(parts[0])
		prefix := strings.ToLower(parts[len(parts)-1])

		switch cmd {
		case "gamemode", "gm":
			if len(parts) == 2 {
				modes := []string{"survival", "creative", "adventure", "spectator"}
				for _, mode := range modes {
					if strings.HasPrefix(mode, prefix) {
						matches = append(matches, mode)
					}
				}
			}
		case "tp", "teleport":
			if len(parts) == 2 {
				s.mu.RLock()
				for _, p := range s.players {
					name := p.Username
					if strings.HasPrefix(strings.ToLower(name), prefix) {
						matches = append(matches, name)
					}
				}
				s.mu.RUnlock()
			}
		case "gamerule":
			if len(parts) == 2 {
				s.mu.RLock()
				for rule := range s.gamerules {
					if strings.HasPrefix(strings.ToLower(rule), prefix) {
						matches = append(matches, rule)
					}
				}
				s.mu.RUnlock()
			} else if len(parts) == 3 {
				ruleName := parts[1]
				s.mu.RLock()
				var currentVal string
				for k, v := range s.gamerules {
					if strings.EqualFold(k, ruleName) {
						currentVal = v
						break
					}
				}
				s.mu.RUnlock()

				// Only suggest true/false if the rule is currently a boolean
				if currentVal == "true" || currentVal == "false" {
					opts := []string{"true", "false"}
					for _, opt := range opts {
						if strings.HasPrefix(opt, prefix) {
							matches = append(matches, opt)
						}
					}
				}
			}
		}
	}

	if len(matches) > 0 {
		s.sendTabComplete(player, matches)
	}
}
