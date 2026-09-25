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

// Everything here talks to Telegram over MTProto, so a test of it would be a
// test of a fake. This file carries the dialling and nothing else, which is
// what sonar.coverage.exclusions names.

package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/query"
	"go.uber.org/zap"
)

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
