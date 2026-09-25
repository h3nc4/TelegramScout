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

package telegram

import (
	"testing"

	"github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/tg"
)

func TestParseID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int64
		ok   bool
	}{
		{"Bot API Channel", "-1001234567890", 1234567890, true},
		{"Standard Chat", "-123456", 123456, true},
		{"Plain ID", "456789", 456789, true},
		{"Username", "@channel", 0, false},
		{"Empty", "", 0, false},
		{"Bot API Prefix Only", "-100", 0, false},
		{"Bot API With Letters", "-100abc", 0, false},
		{"Negative With Letters", "-abc", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := parseID(tt.in)
			if ok != tt.ok {
				t.Fatalf("parseID(%q) ok = %v, want %v", tt.in, ok, tt.ok)
			}
			if ok && id != tt.want {
				t.Errorf("parseID(%q) = %d, want %d", tt.in, id, tt.want)
			}
		})
	}
}

func TestGetPeerID(t *testing.T) {
	tests := []struct {
		name string
		in   tg.InputPeerClass
		want int64
	}{
		{"Channel", &tg.InputPeerChannel{ChannelID: 111}, 111},
		{"Chat", &tg.InputPeerChat{ChatID: 222}, 222},
		{"User", &tg.InputPeerUser{UserID: 333}, 333},
		{"Unsupported", &tg.InputPeerEmpty{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPeerID(tt.in); got != tt.want {
				t.Errorf("getPeerID() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetPeerInfoFromEntities(t *testing.T) {
	entities := peer.NewEntities(
		map[int64]*tg.User{333: {ID: 333, Username: "someone"}},
		map[int64]*tg.Chat{222: {ID: 222, Title: "Group"}},
		map[int64]*tg.Channel{111: {ID: 111, Title: "Channel", Username: "chan"}},
	)

	tests := []struct {
		name     string
		in       tg.InputPeerClass
		title    string
		username string
	}{
		{"Channel", &tg.InputPeerChannel{ChannelID: 111}, "Channel", "chan"},
		{"Chat", &tg.InputPeerChat{ChatID: 222}, "Group", ""},
		{"User", &tg.InputPeerUser{UserID: 333}, "someone", "someone"},
		{"Channel Absent", &tg.InputPeerChannel{ChannelID: 999}, "", ""},
		{"Chat Absent", &tg.InputPeerChat{ChatID: 999}, "", ""},
		{"User Absent", &tg.InputPeerUser{UserID: 999}, "", ""},
		{"Unsupported", &tg.InputPeerEmpty{}, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, username := getPeerInfoFromEntities(tt.in, entities)
			if title != tt.title || username != tt.username {
				t.Errorf("getPeerInfoFromEntities() = (%q, %q), want (%q, %q)",
					title, username, tt.title, tt.username)
			}
		})
	}
}

func TestUpdatePeerCache(t *testing.T) {
	c := &Client{peerCache: make(map[int64]peerInfo)}

	c.updatePeerCache(42, "First", "first")
	if got := c.peerCache[42]; got.Title != "First" || got.Username != "first" {
		t.Fatalf("cache holds %+v after the first write", got)
	}

	// The allowlist is keyed by id, so a second resolution replaces the entry.
	c.updatePeerCache(42, "Renamed", "renamed")
	if got := c.peerCache[42]; got.Title != "Renamed" || got.Username != "renamed" {
		t.Errorf("cache holds %+v after the second write", got)
	}
	if len(c.peerCache) != 1 {
		t.Errorf("cache holds %d entries, want 1", len(c.peerCache))
	}
}
