package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/StoreStation/VibeShitCraft/pkg/server"
)

func main() {
	address := flag.String("address", ":25565", "Server address to listen on")
	maxPlayers := flag.Int("max-players", 20, "Maximum number of players")
	motd := flag.String("motd", "A VibeShitCraft Server", "Server MOTD")
	seed := flag.Int64("seed", 0, "World seed (0 = random)")
	defaultGameMode := flag.String("default-gamemode", "survival", "Default game mode (survival, creative, adventure, spectator)")
	flag.Parse()

	gameMode, ok := server.ParseGameMode(*defaultGameMode)
	if !ok {
		log.Fatalf("Invalid default game mode: %s", *defaultGameMode)
	}

	config := server.Config{
		Address:         *address,
		MaxPlayers:      *maxPlayers,
		MOTD:            *motd,
		Seed:            *seed,
		DefaultGameMode: gameMode,
	}

	srv := server.New(config)
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Printf("VibeShitCraft server started (Minecraft 1.8.9, Protocol 47)")
	log.Printf("Address: %s | Max Players: %d", config.Address, config.MaxPlayers)

	// Wait for interrupt signal or internal shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("Shutting down server (received signal: %v)...", sig)
	case <-srv.StopChan():
		log.Println("Shutting down server (internal)...")
	}

	srv.Stop()
	log.Println("Server stopped.")
}
