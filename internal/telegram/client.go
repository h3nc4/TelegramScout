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
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/telegram/query"
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

type peerInfo struct {
	Title    string
	Username string
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
	if _, err := fmt.Fprintln(a.writer, "2FA Enabled: Cloud password required."); err != nil {
		return "", err
	}
	if _, err := fmt.Fprint(a.writer, "Enter Password: "); err != nil {
		return "", err
	}
	var pwd string
	if _, err := fmt.Fscanln(a.reader, &pwd); err != nil {
		return "", err
	}
	return strings.TrimSpace(pwd), nil
}

// Prompt user to enter login code
func (a *terminalAuthenticator) Code(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
	if _, err := fmt.Fprintln(a.writer, "Action Required: Please enter the login code sent to your Telegram app or via SMS."); err != nil {
		return "", err
	}
	if _, err := fmt.Fprint(a.writer, "Enter Code: "); err != nil {
		return "", err
	}
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
		Logger:         log.WithOptions(zap.IncreaseLevel(zap.WarnLevel)),
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

// Start client, authenticate, resolve peers, and listen for updates
func (c *Client) Run(ctx context.Context) error {
	return c.client.Run(ctx, func(ctx context.Context) error {
		c.log.Info("Telegram client connected to MTProto")

		// Authenticate
		if err := c.authenticate(ctx); err != nil {
			return err
		}

		// Resolve configured chats
		c.log.Info("Resolving configured channels...")
		if err := c.resolveMonitoringPeers(ctx); err != nil {
			c.log.Error("Failed to resolve some peers", zap.Error(err))
		}

		c.log.Info("Client is running and listening for updates...")
		<-ctx.Done()
		return nil
	})
}

func (c *Client) authenticate(ctx context.Context) error {
	status, err := c.client.Auth().Status(ctx)
	if err != nil {
		return fmt.Errorf("auth status check failed: %w", err)
	}

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
	return nil
}

func (c *Client) resolveMonitoringPeers(ctx context.Context) error {
	sender := message.NewSender(c.client.API())

	// Store IDs to look for in dialogs
	// Map: NormalizedID -> OriginalString
	wantedIDs := make(map[int64]string)

	for _, target := range c.cfg.Monitoring.Chats {
		// Check if it's a numeric ID
		if id, ok := parseID(target); ok {
			wantedIDs[id] = target
			continue
		}

		// If it's a username, resolve directly
		if err := c.resolveUsername(ctx, sender, target); err != nil {
			c.log.Warn("Could not resolve chat username", zap.String("chat", target), zap.Error(err))
		}
	}

	// Scan dialogs for the collected numeric IDs
	if len(wantedIDs) > 0 {
		return c.scanDialogsForIDs(ctx, wantedIDs)
	}
	return nil
}

func (c *Client) resolveUsername(ctx context.Context, sender *message.Sender, target string) error {
	cleanTarget := strings.TrimPrefix(target, "@")
	p, err := sender.Resolve(cleanTarget).AsInputPeer(ctx)
	if err != nil {
		return err
	}

	id := getPeerID(p)
	// Optimistically cache using the input username as title
	c.updatePeerCache(id, cleanTarget, cleanTarget)
	c.log.Info("Resolved chat by username", zap.String("target", target), zap.Int64("id", id))
	return nil
}

func (c *Client) scanDialogsForIDs(ctx context.Context, wantedIDs map[int64]string) error {
	c.log.Info("Scanning dialogs to resolve chat IDs...", zap.Int("count", len(wantedIDs)))

	iter := query.GetDialogs(c.client.API()).Iter()
	for iter.Next(ctx) {
		d := iter.Value()
		id := getPeerID(d.Peer)

		if originalTarget, found := wantedIDs[id]; found {
			c.log.Info("Found chat by ID", zap.String("target", originalTarget), zap.Int64("id", id))

			title, username := getPeerInfoFromEntities(d.Peer, d.Entities)
			if title == "" {
				title = originalTarget
			}

			c.updatePeerCache(id, title, username)
			delete(wantedIDs, id)
		}

		if len(wantedIDs) == 0 {
			break
		}
	}

	for _, t := range wantedIDs {
		c.log.Warn("Could not find chat ID in recent dialogs (ensure you have joined the channel/group)", zap.String("target", t))
	}
	return nil
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

// Helpers

func (c *Client) updatePeerCache(id int64, title, username string) {
	c.cacheMux.Lock()
	defer c.cacheMux.Unlock()
	c.peerCache[id] = peerInfo{
		Title:    title,
		Username: username,
	}
}

func parseID(s string) (int64, bool) {
	// Handle -100 prefix (Bot API Channel format)
	if strings.HasPrefix(s, "-100") {
		id, err := strconv.ParseInt(s[4:], 10, 64)
		return id, err == nil
	}
	// Handle - prefix (Standard Chat format)
	if strings.HasPrefix(s, "-") {
		id, err := strconv.ParseInt(s[1:], 10, 64)
		return id, err == nil
	}
	// Normal parsing
	id, err := strconv.ParseInt(s, 10, 64)
	return id, err == nil
}

func getPeerID(p tg.InputPeerClass) int64 {
	switch t := p.(type) {
	case *tg.InputPeerChannel:
		return t.ChannelID
	case *tg.InputPeerChat:
		return t.ChatID
	case *tg.InputPeerUser:
		return t.UserID
	}
	return 0
}

func getPeerInfoFromEntities(p tg.InputPeerClass, e peer.Entities) (string, string) {
	switch t := p.(type) {
	case *tg.InputPeerChannel:
		if ch, ok := e.Channel(t.ChannelID); ok {
			return ch.Title, ch.Username
		}
	case *tg.InputPeerChat:
		if ch, ok := e.Chat(t.ChatID); ok {
			return ch.Title, ""
		}
	case *tg.InputPeerUser:
		if u, ok := e.User(t.UserID); ok {
			return u.Username, u.Username
		}
	}
	return "", ""
}
