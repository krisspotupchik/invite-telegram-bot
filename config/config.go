package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken            string
	DatabaseFile        string
	RewardAmount        float64
	MinWithdrawalAmount float64
	AdminUserIDs        []int64
}

func Load() *Config {
	// Загружаем .env файл, если он существует
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN environment variable is required")
	}

	databaseFile := os.Getenv("DATABASE_FILE")
	if databaseFile == "" {
		databaseFile = "bot_users.db"
	}

	rewardAmount := 0.14
	if envReward := os.Getenv("REWARD_AMOUNT"); envReward != "" {
		if parsed, err := strconv.ParseFloat(envReward, 64); err == nil {
			rewardAmount = parsed
		}
	}

	minWithdrawal := 10.0
	if envMin := os.Getenv("MIN_WITHDRAWAL"); envMin != "" {
		if parsed, err := strconv.ParseFloat(envMin, 64); err == nil {
			minWithdrawal = parsed
		}
	}

	var adminIDs []int64
	if envAdmins := os.Getenv("ADMIN_IDS"); envAdmins != "" {
		for _, idStr := range strings.Split(envAdmins, ",") {
			if id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64); err == nil {
				adminIDs = append(adminIDs, id)
			}
		}
	}

	if len(adminIDs) == 0 {
		log.Fatal("ADMIN_IDS environment variable is required (comma-separated list of Telegram user IDs)")
	}

	return &Config{
		BotToken:            botToken,
		DatabaseFile:        databaseFile,
		RewardAmount:        rewardAmount,
		MinWithdrawalAmount: minWithdrawal,
		AdminUserIDs:        adminIDs,
	}
}

func (c *Config) IsAdmin(userID int64) bool {
	for _, adminID := range c.AdminUserIDs {
		if adminID == userID {
			return true
		}
	}
	return false
}
