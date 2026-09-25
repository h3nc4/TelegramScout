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
	"path/filepath"
	"strings"
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

	// Every credential is required, and two of them have to parse as numbers.
	// A missing one has to be named rather than reaching Telegram as empty.
	rejected := []struct {
		name    string
		drop    string
		set     map[string]string
		wantErr string
	}{
		{name: "Missing API Hash", drop: "TELEGRAM_API_HASH", wantErr: "TELEGRAM_API_HASH is required"},
		{name: "Missing Phone", drop: "TELEGRAM_PHONE", wantErr: "TELEGRAM_PHONE is required"},
		{name: "Missing Bot Token", drop: "TELEGRAM_BOT_TOKEN", wantErr: "TELEGRAM_BOT_TOKEN is required"},
		{name: "Missing Chat ID", drop: "TELEGRAM_CHAT_ID", wantErr: "TELEGRAM_CHAT_ID is required"},
		{
			name:    "API ID Not A Number",
			set:     map[string]string{"TELEGRAM_API_ID": "not-a-number"},
			wantErr: "invalid TELEGRAM_API_ID",
		},
		{
			name:    "Chat ID Not A Number",
			set:     map[string]string{"TELEGRAM_CHAT_ID": "not-a-number"},
			wantErr: "invalid TELEGRAM_CHAT_ID",
		},
	}

	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			env := make(map[string]string)
			maps.Copy(env, baseEnv)
			delete(env, tt.drop)
			maps.Copy(env, tt.set)
			env["TELEGRAM_CONFIG_FILE"] = tmpFile.Name()
			setEnv(env)

			_, err := Load()
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not mention %q", err, tt.wantErr)
			}
		})
	}

	t.Run("Unparsable Rules File", func(t *testing.T) {
		broken := filepath.Join(t.TempDir(), "broken.yaml")
		if err := os.WriteFile(broken, []byte("keywords: [unclosed\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		env := make(map[string]string)
		maps.Copy(env, baseEnv)
		env["TELEGRAM_CONFIG_FILE"] = broken
		setEnv(env)

		if _, err := Load(); err == nil {
			t.Error("expected an error from the malformed file, got nil")
		}
	})

	// With no path given the working directory's config.yaml is read.
	t.Run("Default Rules Path", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"),
			[]byte("chats: [\"only_chat\"]\nkeywords: [\"only\"]\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)

		env := make(map[string]string)
		maps.Copy(env, baseEnv)
		setEnv(env)

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ConfigFilePath != "config.yaml" {
			t.Errorf("config path = %q, want config.yaml", cfg.ConfigFilePath)
		}
		if len(cfg.Monitoring.Chats) != 1 || cfg.Monitoring.Chats[0] != "only_chat" {
			t.Errorf("unexpected chats: %v", cfg.Monitoring.Chats)
		}
	})
}
