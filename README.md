# TelegramScout

A headless Telegram alarm system that monitors specific channels for keywords and notifies you.

## Configuration

Configuration is split between environment variables for credentials and a YAML file for configuration.

### Env Vars

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

### Setting up API Credentials

1. Go to [my.telegram.org](https://my.telegram.org) and log in with your phone number.
2. Navigate to "API Development Tools".
3. Create a new application and note down the `API ID` and `API Hash`.
4. Set `TELEGRAM_API_ID` and `TELEGRAM_API_HASH`.

### Setting up the Bot & Chat ID

1. Create Bot: Talk to [@BotFather](https://t.me/BotFather), create a new bot, and copy the Token.
2. Start Chat: Open your new bot in Telegram and click Start.
3. Get Chat ID: Send any message to [@userinfobot](https://t.me/userinfobot). It will reply with your numeric Id.
4. Set `TELEGRAM_BOT_TOKEN` and `TELEGRAM_CHAT_ID`.

### Session Generation

TelegramScout requires a valid session to run headlessly. Since you cannot interact with the Docker container to enter a login code, you must generate this session locally first.

Ideally, create a .env file with your credentials for easy management.

To generate the session file, run the following commands:

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

Then, follow the prompts to login using the code sent to your Telegram app.

On success, the `session.json` file will be populated with your session data.

Finally, copy its contents and use them as the `TELEGRAM_SESSION` environment variable for running headlessly.

### YAML Config

Define the monitoring rules and performance tuning parameters.

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

## Deployment

### Docker

If you have not generated a session yet, follow the [Session Generation](#session-generation) steps first.

Run the container with your configuration file and environment variables:

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

### Manual

If you prefer to run TelegramScout manually, ensure you have Go installed and set up or enter this repo's Dev Container.

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
