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
	"strconv"
	"strings"

	"github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/tg"
)

type peerInfo struct {
	Title    string
	Username string
}

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
