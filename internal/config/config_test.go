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
	"os"
	"testing"
)

func TestLoadFromEnv(t *testing.T) {
	// Set environment variables for testing
	setEnv := func(vars map[string]string) {
		os.Clearenv()
		for k, v := range vars {
			os.Setenv(k, v)
		}
	}
	defer os.Clearenv()

	// Define base valid configuration
	baseConfig := map[string]string{
		"TELEGRAM_API_ID":    "12345",
		"TELEGRAM_API_HASH":  "abcdef",
		"TELEGRAM_PHONE":     "+1234567890",
		"TELEGRAM_BOT_TOKEN": "bot_token",
		"TELEGRAM_CHAT_ID":   "987654321",
	}

	t.Run("Valid Config", func(t *testing.T) {
		cfgMap := make(map[string]string)
		for k, v := range baseConfig {
			cfgMap[k] = v
		}
		cfgMap["TELEGRAM_PASSWORD"] = "secret_password"
		setEnv(cfgMap)

		cfg, err := LoadFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.AppID != 12345 {
			t.Errorf("expected AppID 12345, got %d", cfg.AppID)
		}
		if cfg.ChatID != 987654321 {
			t.Errorf("expected ChatID 987654321, got %d", cfg.ChatID)
		}
		if cfg.BotToken != "bot_token" {
			t.Errorf("expected BotToken 'bot_token', got %s", cfg.BotToken)
		}
	})

	t.Run("Missing Bot Token", func(t *testing.T) {
		cfgMap := make(map[string]string)
		for k, v := range baseConfig {
			if k != "TELEGRAM_BOT_TOKEN" {
				cfgMap[k] = v
			}
		}
		setEnv(cfgMap)
		_, err := LoadFromEnv()
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("Missing Chat ID", func(t *testing.T) {
		cfgMap := make(map[string]string)
		for k, v := range baseConfig {
			if k != "TELEGRAM_CHAT_ID" {
				cfgMap[k] = v
			}
		}
		setEnv(cfgMap)
		_, err := LoadFromEnv()
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("Invalid Chat ID", func(t *testing.T) {
		cfgMap := make(map[string]string)
		for k, v := range baseConfig {
			cfgMap[k] = v
		}
		cfgMap["TELEGRAM_CHAT_ID"] = "invalid_int"
		setEnv(cfgMap)
		_, err := LoadFromEnv()
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}
