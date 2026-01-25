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

package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/h3nc4/TelegramScout/internal/config"
)

// Define interface for sending alerts
type Notifier interface {
	Send(ctx context.Context, message string) error
}

// Send messages using the Telegram Bot API
type TelegramNotifier struct {
	client  *http.Client
	log     *zap.Logger
	token   string
	chatID  int64
	baseURL string
}

// Create new TelegramNotifier
func New(cfg *config.Config, log *zap.Logger) *TelegramNotifier {
	return &TelegramNotifier{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		log:     log,
		token:   cfg.BotToken,
		chatID:  cfg.ChatID,
		baseURL: "https://api.telegram.org",
	}
}

// Post text message to configured chat
func (t *TelegramNotifier) Send(ctx context.Context, message string) error {
	url := fmt.Sprintf("%s/bot%s/sendMessage", t.baseURL, t.token)

	payload := map[string]interface{}{
		"chat_id": t.chatID,
		"text":    message,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram api returned non-200 status: %d", resp.StatusCode)
	}

	t.log.Info("Notification sent", zap.Int64("chat_id", t.chatID))
	return nil
}
