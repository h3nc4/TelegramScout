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
	"strings"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

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
