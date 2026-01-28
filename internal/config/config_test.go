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
	"maps"
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Setup Environment
	setEnv := func(vars map[string]string) {
		os.Clearenv()
		for k, v := range vars {
			if err := os.Setenv(k, v); err != nil {
				t.Fatalf("failed to set env var %s: %v", k, err)
			}
		}
	}
	defer os.Clearenv()

	// Define base valid configuration
	baseEnv := map[string]string{
		"TELEGRAM_API_ID":    "12345",
		"TELEGRAM_API_HASH":  "abcdef",
		"TELEGRAM_PHONE":     "+1234567890",
		"TELEGRAM_BOT_TOKEN": "bot_token",
		"TELEGRAM_CHAT_ID":   "987654321",
	}

	// Create temporary config file
	configFileContent := `
chats:
  - "cool_channel"
keywords:
  - "urgent"
  - "sale"
`
	tmpFile, err := os.CreateTemp("", "config_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		// Ignore error on remove in cleanup
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.Write([]byte(configFileContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	t.Run("Valid Full Config", func(t *testing.T) {
		env := make(map[string]string)
		maps.Copy(env, baseEnv)
		env["TELEGRAM_CONFIG_FILE"] = tmpFile.Name()
		setEnv(env)

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.AppID != 12345 {
			t.Errorf("expected AppID 12345, got %d", cfg.AppID)
		}
		if len(cfg.Monitoring.Chats) != 1 || cfg.Monitoring.Chats[0] != "cool_channel" {
			t.Errorf("unexpected chats config: %v", cfg.Monitoring.Chats)
		}
		if len(cfg.Monitoring.Keywords) != 2 {
			t.Errorf("expected 2 keywords, got %d", len(cfg.Monitoring.Keywords))
		}
	})

	t.Run("Missing Env Var", func(t *testing.T) {
		env := make(map[string]string)
		for k, v := range baseEnv {
			if k != "TELEGRAM_API_ID" {
				env[k] = v
			}
		}
		env["TELEGRAM_CONFIG_FILE"] = tmpFile.Name()
		setEnv(env)

		_, err := Load()
		if err == nil {
			t.Error("expected error due to missing API ID, got nil")
		}
	})

	t.Run("Missing Config File", func(t *testing.T) {
		env := make(map[string]string)
		maps.Copy(env, baseEnv)
		env["TELEGRAM_CONFIG_FILE"] = "non_existent.yaml"
		setEnv(env)

		_, err := Load()
		if err == nil {
			t.Error("expected error due to missing config file, got nil")
		}
	})
}
