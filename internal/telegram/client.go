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
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/gotd/log/logzap"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"

	"github.com/h3nc4/TelegramScout/internal/config"
	"github.com/h3nc4/TelegramScout/internal/model"
)

// Wrap MTProto client
type Client struct {
	client     *telegram.Client
	log        *zap.Logger
	cfg        *config.Config
	msgChan    chan<- model.Message
	dispatcher tg.UpdateDispatcher

	// Cache for resolved peer info (ID -> Title/Username)
	// Also acts as the ALLOWLIST for monitoring.
	peerCache map[int64]peerInfo
	cacheMux  sync.RWMutex

	stdin  io.Reader
	stdout io.Writer
}

// Create new Telegram client instance
func NewClient(cfg *config.Config, log *zap.Logger, msgChan chan<- model.Message) (*Client, error) {
	var storage session.Storage
	if cfg.Session != "" {
		storage = &memorySession{data: []byte(cfg.Session)}
	} else {
		storage = &session.FileStorage{Path: "session.json"}
	}

	// Setup update dispatcher
	d := tg.NewUpdateDispatcher()

	opts := telegram.Options{
		// Reduce log noise from the library
		Logger:         logzap.New(log.WithOptions(zap.IncreaseLevel(zap.WarnLevel))),
		SessionStorage: storage,
		UpdateHandler:  d,
	}

	client := telegram.NewClient(cfg.AppID, cfg.AppHash, opts)
	c := &Client{
		client:     client,
		log:        log,
		cfg:        cfg,
		msgChan:    msgChan,
		dispatcher: d,
		peerCache:  make(map[int64]peerInfo),
		stdin:      os.Stdin,
		stdout:     os.Stdout,
	}

	// Register handlers
	d.OnNewChannelMessage(c.handleNewChannelMessage)
	d.OnNewMessage(c.handleNewMessage)

	return c, nil
}

func (c *Client) handleNewChannelMessage(ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {
	msg, ok := u.Message.(*tg.Message)
	if !ok {
		return nil
	}
	return c.emitMessage(ctx, msg, e)
}

func (c *Client) handleNewMessage(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
	msg, ok := u.Message.(*tg.Message)
	if !ok {
		return nil
	}
	return c.emitMessage(ctx, msg, e)
}

func (c *Client) emitMessage(ctx context.Context, msg *tg.Message, entities tg.Entities) error {
	var chatID int64
	var title, username string

	// Handle different Peer types
	switch p := msg.PeerID.(type) {
	case *tg.PeerChannel:
		chatID = p.ChannelID
		if ch, ok := entities.Channels[chatID]; ok {
			title = ch.Title
			username = ch.Username
		}
	case *tg.PeerChat:
		chatID = p.ChatID
		if ch, ok := entities.Chats[chatID]; ok {
			title = ch.Title
		}
	case *tg.PeerUser:
		chatID = p.UserID
	}

	// Only process messages from chats resolved.
	c.cacheMux.RLock()
	info, allowed := c.peerCache[chatID]
	c.cacheMux.RUnlock()

	if !allowed {
		// Ignore messages from non-monitored chats
		return nil
	}

	// Use cached info if entity data was missing
	if title == "" {
		title = info.Title
	}
	if username == "" {
		username = info.Username
	}

	// Construct Link
	link := ""
	if username != "" {
		link = fmt.Sprintf("https://t.me/%s/%d", username, msg.ID)
	} else {
		// Private link format
		link = fmt.Sprintf("https://t.me/c/%d/%d", chatID, msg.ID)
	}

	c.msgChan <- model.Message{
		ID:        msg.ID,
		ChatID:    chatID,
		ChatTitle: title,
		Username:  username,
		Text:      msg.Message,
		Date:      time.Unix(int64(msg.Date), 0),
		Link:      link,
	}

	return nil
}
