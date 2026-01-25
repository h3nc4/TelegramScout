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
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/query"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"

	"github.com/h3nc4/TelegramScout/internal/config"
)

// Define interface for interacting with Telegram
type ScoutClient interface {
	Run(ctx context.Context, handler func(ctx context.Context, api *tg.Client) error) error
}

// Wrap MTProto client
type Client struct {
	client *telegram.Client
	log    *zap.Logger
	cfg    *config.Config
	stdin  io.Reader
	stdout io.Writer
}

// Implement session.Storage for in-memory handling
type memorySession struct {
	data []byte
	mux  sync.RWMutex
}

// Retrieve session data from memory
func (m *memorySession) LoadSession(ctx context.Context) ([]byte, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()
	if len(m.data) == 0 {
		return nil, session.ErrNotFound
	}
	// Return a copy to ensure thread safety
	out := make([]byte, len(m.data))
	copy(out, m.data)
	return out, nil
}

// Store session data in memory
func (m *memorySession) StoreSession(ctx context.Context, data []byte) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.data = make([]byte, len(data))
	copy(m.data, data)
	return nil
}

// Implement auth.UserAuthenticator for interactive login
type terminalAuthenticator struct {
	phone    string
	password string
	reader   io.Reader
	writer   io.Writer
}

func (a *terminalAuthenticator) Phone(ctx context.Context) (string, error) {
	return a.phone, nil
}

func (a *terminalAuthenticator) Password(ctx context.Context) (string, error) {
	if a.password != "" {
		return a.password, nil
	}
	fmt.Fprintln(a.writer, "2FA Enabled: Cloud password required.")
	fmt.Fprint(a.writer, "Enter Password: ")

	var pwd string
	if _, err := fmt.Fscanln(a.reader, &pwd); err != nil {
		return "", err
	}
	return strings.TrimSpace(pwd), nil
}

// Prompt user to enter login code
func (a *terminalAuthenticator) Code(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
	fmt.Fprintln(a.writer, "Action Required: Please enter the login code sent to your Telegram app or via SMS.")
	fmt.Fprint(a.writer, "Enter Code: ")

	var code string
	if _, err := fmt.Fscanln(a.reader, &code); err != nil {
		return "", err
	}
	return strings.TrimSpace(code), nil
}

func (a *terminalAuthenticator) AcceptTermsOfService(ctx context.Context, tos tg.HelpTermsOfService) error {
	return nil
}

func (a *terminalAuthenticator) SignUp(ctx context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, errors.New("signup not supported in TelegramScout")
}

// Create new Telegram client instance
func NewClient(cfg *config.Config, log *zap.Logger) (*Client, error) {
	// Initialize session storage
	var storage session.Storage

	if cfg.Session != "" {
		storage = &memorySession{
			data: []byte(cfg.Session),
		}
	} else {
		storage = &session.FileStorage{
			Path: "session.json",
		}
	}

	opts := telegram.Options{
		Logger:         log,
		SessionStorage: storage,
	}

	client := telegram.NewClient(cfg.AppID, cfg.AppHash, opts)
	return &Client{
		client: client,
		log:    log,
		cfg:    cfg,
		stdin:  os.Stdin,
		stdout: os.Stdout,
	}, nil
}

// Start client connection and execute provided logic
func (c *Client) Run(ctx context.Context, action func(ctx context.Context, api *tg.Client) error) error {
	return c.client.Run(ctx, func(ctx context.Context) error {
		// Check auth status
		status, err := c.client.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("auth status check failed: %w", err)
		}

		// If not authorized, start auth flow
		if !status.Authorized {
			c.log.Info("Starting new authentication flow")

			authenticator := &terminalAuthenticator{
				phone:    c.cfg.Phone,
				password: c.cfg.Password,
				reader:   c.stdin,
				writer:   c.stdout,
			}

			flow := auth.NewFlow(authenticator, auth.SendCodeOptions{})

			if err := c.client.Auth().IfNecessary(ctx, flow); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
		} else {
			c.log.Info("Using existing session")
		}

		return action(ctx, c.client.API())
	})
}

// Retrieve recent messages from channel
func FetchChannelHistory(ctx context.Context, api *tg.Client, target string, limit int) ([]string, int64, error) {
	p, id, err := resolvePeer(ctx, api, target)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to resolve peer %q: %w", target, err)
	}

	res, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:  p,
		Limit: limit,
	})
	if err != nil {
		return nil, id, fmt.Errorf("failed to get history: %w", err)
	}

	var msgs []string
	var messages []tg.MessageClass

	switch m := res.(type) {
	case *tg.MessagesMessages:
		messages = m.Messages
	case *tg.MessagesMessagesSlice:
		messages = m.Messages
	case *tg.MessagesChannelMessages:
		messages = m.Messages
	default:
		return nil, id, errors.New("unknown message response type")
	}

	for _, m := range messages {
		if msg, ok := m.(*tg.Message); ok {
			if msg.Message != "" {
				msgs = append(msgs, fmt.Sprintf("[%d] %s", msg.Date, msg.Message))
			}
		}
	}

	return msgs, id, nil
}

// Determine input peer from string (username) or int64 (ID)
func resolvePeer(ctx context.Context, api *tg.Client, target string) (tg.InputPeerClass, int64, error) {
	// Try to parse an integer ID
	if id, err := strconv.ParseInt(target, 10, 64); err == nil {
		return findPeerByID(ctx, api, id)
	}

	// Treat as username
	sender := message.NewSender(api)
	inputPeer, err := sender.Resolve(target).AsInputPeer(ctx)
	if err != nil {
		return nil, 0, err
	}

	// Extract ID for logging
	var resolvedID int64
	switch p := inputPeer.(type) {
	case *tg.InputPeerChannel:
		resolvedID = p.ChannelID
	case *tg.InputPeerUser:
		resolvedID = p.UserID
	case *tg.InputPeerChat:
		resolvedID = p.ChatID
	}

	return inputPeer, resolvedID, nil
}

// Iterate over dialogs to find chat with matching ID
func findPeerByID(ctx context.Context, api *tg.Client, targetID int64) (tg.InputPeerClass, int64, error) {
	// Normalize ID
	searchID := targetID
	if searchID < 0 {
		strID := fmt.Sprintf("%d", targetID)
		if strings.HasPrefix(strID, "-100") {
			parsed, _ := strconv.ParseInt(strID[4:], 10, 64)
			searchID = parsed
		} else {
			searchID = -searchID
		}
	}

	// Iterate dialogs to find the chat
	iter := query.GetDialogs(api).Iter()
	for iter.Next(ctx) {
		d := iter.Value()

		var currentID int64

		switch p := d.Peer.(type) {
		case *tg.InputPeerUser:
			currentID = p.UserID
		case *tg.InputPeerChat:
			currentID = p.ChatID
		case *tg.InputPeerChannel:
			currentID = p.ChannelID
		}

		if currentID == searchID {
			return d.Peer, currentID, nil
		}
	}

	if err := iter.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating dialogs: %w", err)
	}

	return nil, 0, fmt.Errorf("peer with ID %d not found in recent dialogs (ensure you have joined/started the chat)", targetID)
}
