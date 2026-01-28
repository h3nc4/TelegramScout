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

package main

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/h3nc4/TelegramScout/internal/config"
	"github.com/h3nc4/TelegramScout/internal/model"
	"github.com/h3nc4/TelegramScout/internal/scout"
)

// Implement notifier.Notifier for testing
type MockNotifier struct {
	LastMessage string
	CallCount   int
}

func (m *MockNotifier) Send(ctx context.Context, message string) error {
	m.LastMessage = message
	m.CallCount++
	return nil
}

// Integration-like test for the wiring of components
func TestWiring(t *testing.T) {
	log := zap.NewNop()
	cfg := &config.Config{
		Monitoring: config.MonitoringRules{
			Chats:    []string{"test_chat"},
			Keywords: []string{"alert"},
		},
	}

	notif := &MockNotifier{}
	s := scout.New(cfg, notif, log)
	msgChan := make(chan model.Message, 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Scout
	go s.Start(ctx, msgChan)

	// Simulate incoming message matching keyword
	msgChan <- model.Message{
		ID:        1,
		ChatID:    100,
		ChatTitle: "test_chat",
		Text:      "This is an ALERT message",
		Date:      time.Now(),
	}

	// Allow time for processing
	time.Sleep(100 * time.Millisecond)

	if notif.CallCount != 1 {
		t.Errorf("expected 1 notification, got %d", notif.CallCount)
	}
}
