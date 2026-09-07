# TelegramScout

A headless Telegram alarm system that monitors specific channels for keywords and notifies you.

## Run with Docker

```bash
docker run -d \
  --name telegram-scout \
  --restart always \
  -v "${PWD}/config.yaml:/app/config.yaml:ro" \
  -e TELEGRAM_API_ID='YOUR_API_ID' \
  -e TELEGRAM_API_HASH='YOUR_API_HASH' \
  -e TELEGRAM_PHONE='+1234567890' \
  -e TELEGRAM_PASSWORD='YOUR_2FA_PASSWORD' \
  -e TELEGRAM_SESSION='{"version":1,"data":...}' \
  -e TELEGRAM_BOT_TOKEN='YOUR_BOT_TOKEN' \
  -e TELEGRAM_CHAT_ID='YOUR_CHAT_ID' \
  h3nc4/telegram-scout
```

That command needs a `config.yaml` naming the chats and keywords to watch, plus a
`TELEGRAM_SESSION` value. The session matters because a container has no way to prompt you for
the login code Telegram sends.

Setting both up is one-time work. Go through the sections below in order and you will end up with
every value the command above expects.

1. [API credentials](#api-credentials)
2. [Bot token and chat id](#bot-token-and-chat-id)
3. [Session string](#session-string)
4. [config.yaml](#configyaml)

## API credentials

1. Log in at [my.telegram.org](https://my.telegram.org) with your phone number.
2. Open "API Development Tools".
3. Create an application and keep the `API ID` and `API Hash` it shows you.

Those two values are `TELEGRAM_API_ID` and `TELEGRAM_API_HASH`.

## Bot token and chat id

1. Talk to [@BotFather](https://t.me/BotFather) and create a new bot. Copy the token it hands back.
2. Open your new bot in Telegram and press Start.
3. Send any message to [@userinfobot](https://t.me/userinfobot). It replies with your numeric id.

Those two values are `TELEGRAM_BOT_TOKEN` and `TELEGRAM_CHAT_ID`.

## Session string

Telegram signs you in with a code sent to your app, so the first login cannot happen inside a
container. Generate the session on your own machine, then pass the result in as an environment
variable.

A `.env` file makes the credentials easier to manage here, though it is not required.

```bash
touch session.json
docker run \
  -e TELEGRAM_API_ID='YOUR_API_ID' \
  -e TELEGRAM_API_HASH='YOUR_API_HASH' \
  -e TELEGRAM_PHONE='+1234567890' \
  -e TELEGRAM_PASSWORD='YOUR_2FA_PASSWORD' \
  -e TELEGRAM_BOT_TOKEN='YOUR_BOT_TOKEN' \
  -e TELEGRAM_CHAT_ID='YOUR_CHAT_ID' \
  -u "$(id -u):$(id -g)" \
  -v "${PWD}:/target" -w /target \
  --rm -it h3nc4/telegram-scout
```

Follow the prompts and enter the code Telegram sends you. On success `session.json` holds your
session data, and its contents are the value of `TELEGRAM_SESSION`.

## config.yaml

Define the chats to watch and the keywords that raise an alert.

```yaml
chats: # List of chat usernames or IDs to monitor
  - "example_channel"
  - "example_girlfriend"
  - "example_bro"
  - -1001803446893
  - 1710595474

keywords: # Keywords to trigger alerts (case-insensitive)
  - "urgent"
  - "im home alone"
  - "hey bro lets party"
  - "*"
  - "rtx 5070"
  - "re:(?i)urgent|important" # Case insensitive 'urgent' OR 'important'
  - "re:\$\d{3,}"             # Matches prices
```

## Environment variables

Credentials come from the environment. The monitoring rules come from `config.yaml`.

| Variable             | Description                                              | Required |
| -------------------- | -------------------------------------------------------- | -------- |
| `TELEGRAM_PHONE`     | Phone number with country code (e.g., `+1234567890`)     | Yes      |
| `TELEGRAM_PASSWORD`  | Cloud password (2FA) if enabled                          | No*      |
| `TELEGRAM_API_ID`    | App ID from [my.telegram.org](https://my.telegram.org)   | Yes      |
| `TELEGRAM_API_HASH`  | App Hash from [my.telegram.org](https://my.telegram.org) | Yes      |
| `TELEGRAM_BOT_TOKEN` | Token from [@BotFather](https://t.me/BotFather)          | Yes      |
| `TELEGRAM_CHAT_ID`   | User or Group ID to receive alerts                       | Yes      |
| `TELEGRAM_SESSION`   | JSON session string                                      | No*      |

*\* `TELEGRAM_SESSION` is required for headless/Docker operation. `TELEGRAM_PASSWORD` is required if 2FA is enabled.*

## Running without Docker

You still need the values from the sections above. Make sure you have Go installed, or enter this
repository's Dev Container.

```bash
# Install dependencies
go mod download

# Run
go run cmd/telegram-scout/main.go
```

## License

TelegramScout is free software: you can redistribute it and/or modify it under the terms of the GNU Affero General Public License as published by the Free Software Foundation, either version 3 of the License, or (at your option) any later version.

TelegramScout is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License along with TelegramScout. If not, see <https://www.gnu.org/licenses/>.
