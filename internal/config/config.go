/*
 * Copyright (C) 2026  Henrique Almeida
 * This file is part of TelegramScout.
 *
 * TelegramScout is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published
 * by the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * TelegramScout is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with TelegramScout.  If not, see <https://www.gnu.org/licenses/>.
 */

package config

import (
	"fmt"
	"os"
	"strconv"
)

// Hold application configuration
type Config struct {
	AppID          int
	AppHash        string
	Phone          string
	Password       string // 2FA Cloud Password
	Session        string
	BotToken       string // Telegram Bot API Token
	ChatID         int64  // Target Chat ID for notifications
	TargetChannel  string
	Limit          int
	ConfigFilePath string
}

// Populate Config struct from environment variables
func LoadFromEnv() (*Config, error) {
	appIDStr := os.Getenv("TELEGRAM_API_ID")
	if appIDStr == "" {
		return nil, fmt.Errorf("TELEGRAM_API_ID is required")
	}

	appID, err := strconv.Atoi(appIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid TELEGRAM_API_ID: %w", err)
	}

	appHash := os.Getenv("TELEGRAM_API_HASH")
	if appHash == "" {
		return nil, fmt.Errorf("TELEGRAM_API_HASH is required")
	}

	phone := os.Getenv("TELEGRAM_PHONE")
	if phone == "" {
		return nil, fmt.Errorf("TELEGRAM_PHONE is required")
	}

	// Bot Configuration
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required for notifications")
	}

	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	if chatIDStr == "" {
		return nil, fmt.Errorf("TELEGRAM_CHAT_ID is required for notifications")
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid TELEGRAM_CHAT_ID: %w", err)
	}

	// Optional configurations
	password := os.Getenv("TELEGRAM_PASSWORD")
	session := os.Getenv("TELEGRAM_SESSION")
	targetChannel := os.Getenv("TELEGRAM_TARGET_CHANNEL")
	if targetChannel == "" {
		targetChannel = "telegram" // Default channel
	}

	return &Config{
		AppID:         appID,
		AppHash:       appHash,
		Phone:         phone,
		Password:      password,
		Session:       session,
		BotToken:      botToken,
		ChatID:        chatID,
		TargetChannel: targetChannel,
		Limit:         50, // Default fetch limit
	}, nil
}
