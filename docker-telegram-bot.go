package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type DockerBot struct {
	bot          *tgbotapi.BotAPI
	dockerClient *client.Client
	allowedUser  int64
	logger       zerolog.Logger
}

func main() {
	// Initialize zerolog
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Get configuration from environment variables
	botToken := os.Getenv("TOKEN")
	if botToken == "" {
		log.Fatal().Msg("TOKEN environment variable is required")
	}

	allowedUserStr := os.Getenv("USER_ID")
	if allowedUserStr == "" {
		log.Fatal().Msg("USER_ID environment variable is required")
	}

	allowedUser, err := strconv.ParseInt(allowedUserStr, 10, 64)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid USER_ID")
	}

	// Initialize Telegram bot
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Telegram bot")
	}

	// Initialize Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Docker client")
	}
	defer dockerClient.Close()

	dockerBot := &DockerBot{
		bot:          bot,
		dockerClient: dockerClient,
		allowedUser:  allowedUser,
		logger:       log.With().Str("component", "docker-telegram-bot").Logger(),
	}

	dockerBot.logger.Info().
		Str("bot_username", bot.Self.UserName).
		Int64("allowed_user", allowedUser).
		Msg("Docker Telegram Bot started")

	// Start bot
	dockerBot.run()
}

func (db *DockerBot) run() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := db.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		// Check if user is authorized
		if update.Message.From.ID != db.allowedUser {
			db.logger.Warn().
				Int64("user_id", update.Message.From.ID).
				Str("username", update.Message.From.UserName).
				Msg("Unauthorized access attempt")

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Unauthorized access")
			db.bot.Send(msg)
			continue
		}

		db.logger.Info().
			Int64("user_id", update.Message.From.ID).
			Str("command", update.Message.Text).
			Msg("Processing command")

		db.handleCommand(update.Message)
	}
}

func (db *DockerBot) handleCommand(message *tgbotapi.Message) {
	command := strings.Fields(message.Text)
	if len(command) == 0 {
		return
	}

	var response string

	switch command[0] {
	case "/status":
		response = db.handleList(false)
	case "/list":
		response = db.handleList(false)
	case "/detailed":
		response = db.handleList(true)
	case "/start":
		if len(command) < 2 {
			response = db.handleList(false)
		} else {
			response = db.handleStartContainer(command[1])
		}
	case "/stop":
		if len(command) < 2 {
			response = db.handleList(false)
		} else {
			response = db.handleStopContainer(command[1])
		}
	case "/restart":
		if len(command) < 2 {
			response = "❌ Usage: /restartcontainer <container_name_or_id>"
		} else {
			response = db.handleRestartContainer(command[1])
		}
	case "/logs":
		if len(command) < 2 {
			response = db.handleList(false)
		} else {
			lines := 10 // default
			if len(command) > 2 {
				if l, err := strconv.Atoi(command[2]); err == nil && l > 0 && l <= 1000 {
					lines = l
				}
			}
			response = db.handleLogs(command[1], lines)
		}
	case "/help":
		response = db.handleHelp()
	default:
		response = "❌ Unknown command. Use /help to see available commands."
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, response)
	msg.ParseMode = "Markdown"
	msg.DisableWebPagePreview = true

	if _, err := db.bot.Send(msg); err != nil {
		db.logger.Error().Err(err).Msg("Failed to send message")
	}
}

func (db *DockerBot) handleStart() string {
	return `🤖 *Docker Management Bot*

Welcome! I can help you manage Docker containers.

Use /help to see available commands.`
}

func (db *DockerBot) handleHelp() string {
	return `🤖 *Available Commands:*

• */list* - List all containers
• */detailed* - List all containers with extra details
• */start* <name> - Start a container
• */stop* <name> - Stop a container
• */restart* <name> - Restart a container
• */logs* <name> [lines] - Show container logs (default: 10 lines, max: 1000)
• */help* - Show this help message
`
}

func (db *DockerBot) handleList(detailed bool) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	containers, err := db.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		db.logger.Error().Err(err).Msg("Failed to list containers")
		return "❌ Failed to list containers"
	}

	if len(containers) == 0 {
		return "📦 No containers found"
	}

	var response strings.Builder
	response.WriteString("📦 *Docker Containers:*\n\n")

	for _, container := range containers {
		status := "❓ Unknown"
		if container.State == "running" {
			status = fmt.Sprintf("✅ %s", container.Status)
		} else if container.State == "exited" {
			status = fmt.Sprintf("⛔ %s", container.Status)
		} else if container.State == "removing" {
			status = fmt.Sprintf("⛏️ %s", container.Status)
		} else if container.State == "dead" {
			status = fmt.Sprintf("💀 %s", container.Status)
		} else if container.State == "created" {
			status = fmt.Sprintf("📄 %s", container.Status)
		} else if container.State == "restarting" {
			status = fmt.Sprintf("♻️ %s", container.Status)
		} else if container.State == "paused" {
			status = fmt.Sprintf("⏸️ %s", container.Status)
		}

		name := strings.TrimPrefix(container.Names[0], "/")

		response.WriteString(fmt.Sprintf("*%s* %s\n", name, status))
		if detailed {
			response.WriteString(fmt.Sprintf("%s (ID: %s)\n\n", container.Image, container.ID[:12]))
		}
	}

	return response.String()
}

func (db *DockerBot) handleStartContainer(containerName string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First check if container exists
	containers, err := db.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		db.logger.Error().Err(err).Str("container", containerName).Msg("Failed to list containers")
		return "❌ Failed to check container status"
	}

	var containerID string
	var found bool

	for _, container := range containers {
		name := strings.TrimPrefix(container.Names[0], "/")
		if name == containerName || container.ID[:12] == containerName || container.ID == containerName {
			containerID = container.ID
			found = true

			if container.State == "running" {
				return fmt.Sprintf("ℹ️ Container `%s` is already running", containerName)
			}
			break
		}
	}

	if !found {
		return fmt.Sprintf("❌ Container `%s` not found", containerName)
	}

	// Start the container
	err = db.dockerClient.ContainerStart(ctx, containerID, container.StartOptions{})
	if err != nil {
		db.logger.Error().Err(err).Str("container", containerName).Msg("Failed to start container")
		return fmt.Sprintf("❌ Failed to start container `%s`", containerName)
	}

	db.logger.Info().Str("container", containerName).Msg("Container started successfully")
	return fmt.Sprintf("✅ Container `%s` started successfully", containerName)
}

func (db *DockerBot) handleStopContainer(containerName string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First check if container exists and is running
	containers, err := db.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		db.logger.Error().Err(err).Str("container", containerName).Msg("Failed to list containers")
		return "❌ Failed to check container status"
	}

	var containerID string
	var found bool

	for _, container := range containers {
		name := strings.TrimPrefix(container.Names[0], "/")
		if name == containerName || container.ID[:12] == containerName || container.ID == containerName {
			containerID = container.ID
			found = true

			if container.State != "running" {
				return fmt.Sprintf("ℹ️ Container `%s` is not running", containerName)
			}
			break
		}
	}

	if !found {
		return fmt.Sprintf("❌ Container `%s` not found", containerName)
	}

	// Stop the container with 10 second timeout
	timeout := 10
	err = db.dockerClient.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
	if err != nil {
		db.logger.Error().Err(err).Str("container", containerName).Msg("Failed to stop container")
		return fmt.Sprintf("❌ Failed to stop container `%s`", containerName)
	}

	db.logger.Info().Str("container", containerName).Msg("Container stopped successfully")
	return fmt.Sprintf("✅ Container `%s` stopped successfully", containerName)
}

func (db *DockerBot) handleRestartContainer(containerName string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// First check if container exists
	containers, err := db.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		db.logger.Error().Err(err).Str("container", containerName).Msg("Failed to list containers")
		return "❌ Failed to check container status"
	}

	var containerID string
	var found bool

	for _, container := range containers {
		name := strings.TrimPrefix(container.Names[0], "/")
		if name == containerName || container.ID[:12] == containerName || container.ID == containerName {
			containerID = container.ID
			found = true
			break
		}
	}

	if !found {
		return fmt.Sprintf("❌ Container `%s` not found", containerName)
	}

	// Restart the container with 30 second timeout
	timeout := 30
	err = db.dockerClient.ContainerRestart(ctx, containerID, container.StopOptions{Timeout: &timeout})
	if err != nil {
		db.logger.Error().Err(err).Str("container", containerName).Msg("Failed to restart container")
		return fmt.Sprintf("❌ Failed to restart container `%s`", containerName)
	}

	db.logger.Info().Str("container", containerName).Msg("Container restarted successfully")
	return fmt.Sprintf("🔄 Container `%s` restarted successfully", containerName)
}

func (db *DockerBot) handleLogs(containerName string, lines int) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First check if container exists
	containers, err := db.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		db.logger.Error().Err(err).Str("container", containerName).Msg("Failed to list containers")
		return "❌ Failed to check container status"
	}

	var containerID string
	var found bool

	for _, container := range containers {
		name := strings.TrimPrefix(container.Names[0], "/")
		if name == containerName || container.ID[:12] == containerName || container.ID == containerName {
			containerID = container.ID
			found = true
			break
		}
	}

	if !found {
		return fmt.Sprintf("❌ Container `%s` not found", containerName)
	}

	// Get container logs
	logOptions := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       strconv.Itoa(lines),
		Timestamps: true,
	}

	logs, err := db.dockerClient.ContainerLogs(ctx, containerID, logOptions)
	if err != nil {
		db.logger.Error().Err(err).Str("container", containerName).Msg("Failed to get container logs")
		return fmt.Sprintf("❌ Failed to get logs for container `%s`", containerName)
	}
	defer logs.Close()

	var logContent strings.Builder
	scanner := bufio.NewScanner(logs)
	lineCount := 0

	for scanner.Scan() && lineCount < lines {
		line := scanner.Text()
		// Docker logs include a header (8 bytes) that we need to skip
		if len(line) > 8 {
			logContent.WriteString(line[8:] + "\n")
		}
		lineCount++
	}

	if logContent.Len() == 0 {
		return fmt.Sprintf("📋 No logs found for container `%s`", containerName)
	}

	response := fmt.Sprintf("📋 *Logs for container `%s`* (last %d lines):\n\n```\n%s```",
		containerName, lineCount, logContent.String())

	// Telegram message limit is 4096 characters
	if len(response) > 4000 {
		response = response[:4000] + "\n...\n```\n\n⚠️ *Log output truncated due to length limit*"
	}

	return response
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		if hours == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd %dh", days, hours)
	}
}
