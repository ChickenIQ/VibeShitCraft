# Simple Minecraft 1.8 Server in GoLang 🚀

A lightweight Minecraft 1.8.9 (Protocol 47) server written in Go. Supports offline-mode authentication, flat world generation, chat, and multiplayer.

## Features

- **Protocol 47** – Compatible with Minecraft 1.8.x clients
- **Server List Ping** – Shows MOTD, player count, and version in the multiplayer browser
- **Offline Mode Login** – No authentication server required
- **Flat World** – Superflat world with bedrock, dirt, and grass layers
- **Chat** – Players can send and receive chat messages
- **Multiplayer** – Players can see each other, with entity movement and head rotation
- **Keep Alive** – Automatic keep-alive to maintain connections
- **Configurable** – Address, max players, and MOTD via command-line flags

## Build

```bash
go build -o vibeshitcraft ./cmd/server/
```

## Run

```bash
./vibeshitcraft
```

### Options

| Flag           | Default                  | Description                  |
|----------------|--------------------------|------------------------------|
| `-address`     | `:25565`                 | Server listen address        |
| `-max-players` | `20`                     | Maximum player count         |
| `-motd`        | `A VibeShitCraft Server` | Message of the day           |

Example:

```bash
./vibeshitcraft -address :25565 -max-players 50 -motd "Welcome to VibeShitCraft!"
```

## Test

```bash
go test ./...
```

## Project Structure

```
cmd/server/        – Server entry point
pkg/protocol/      – Minecraft protocol types and packet framing
pkg/server/        – Server logic, connection handling, game loop
pkg/world/         – World/chunk generation
pkg/chat/          – Chat message formatting
```
