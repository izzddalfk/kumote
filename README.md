# 🤖 Kumote - Remote Work Assistant

**KumoTe** (雲手) - Japanese for "Cloud Hand" - Your remote development companion that extends your coding capabilities through the cloud.

A Telegram-based remote development assistant that connects you to your local projects through AI. Query your codebase, browse project structures, and get intelligent analysis of your development work from anywhere - all through simple Telegram messages.

# Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Usage Examples](#-usage-examples)
- [Notices](#notices)
- [Planned Features](#planned-features)

## ✨ Features

- 🔐 **Secure Remote Access** - Access your development machine through encrypted Telegram messages using your own Bot
- 🤖 **AI-Powered Analysis** - Leverage Claude Code CLI for intelligent code analysis and exploration
- 📁 **Smart Project Discovery** - Automatically discovers and indexes your development projects
- 📊 **Command History & Metrics** - SQLite-based storage for command history and execution metrics
- ⚡ **Asynchronous Processing** - Efficient processing with webhook and background command execution

## 🏗️ Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Telegram Bot  │───▶│ Cloudflare      │───▶│   Go Server     │
│   (User Input)  │    │   Tunnel        │    │   (Wrapper)     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                        │
                                               ┌─────────────────┐
                                               │ Claude Code CLI │
                                               │  (AI Engine)    │
                                               └─────────────────┘
                                                        │
                                               ┌─────────────────┐
                                               │  Development    │
                                               │    Projects     │
                                               │  (~/Development)│
                                               └─────────────────┘
```

## 🚀 Quick Start

### Prerequisites

- Go 1.24+
- Telegram Bot Token ([Get one from @BotFather](https://t.me/botfather))
- Claude Code CLI installed. Check this [documentation](https://docs.anthropic.com/en/docs/claude-code/setup).
- Tunneling service (e.g., Cloudflare Tunnel)

> Note: To use Claude Code, you need at least Pro subscription to Claude.

### 1. Download Kumote

```bash
git clone https://github.com/izzddalfk/kumote.git
cd kumote

# Copy environment template
cp env.example .env
```

### 2. Setup Kumote

Fill all environment variables with your own values in .env file. Then open project index file in `/data/projects-index.json`.

Add any projects that you want to Kumote work with. You can add multiple projects as many as you want by following format that pre-defined in that file.

Keep in mind you need to give full path to the project directory. And it's better to give name to the projects with any name that you naturally use. Because later Kumote will determine which project directory that you want to work with by the name.

### 3. Run Kumote

```bash
# Start the application
make run
```

It should run in port 3377. Check with

```http
curl http://localhost:3377
```

If you get response like below, it means you're good to go!

```json
{
  "success": true,
  "data": "It's running!",
  "timestamp": 1752982338
}
```

> Development or customize Kumote with auto-refresh for changes in the codebase
> run this command instead
> `make dev`

### 4. Setup Tunnel

In order Telegram can connect to Kumote in your local machine, the fastest and cheapest way is to use tunneling. In this example, we will use Cloudflare Tunnel. You can use any other tunneling service as well.

```bash
# Install cloudflared
# https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/get-started/

# Then run:
cloudflared tunnel run --token {YOUR_CLOUDFLARE_TOKEN}
```

If tunneling is successful, you should have public-internet that provided by the tunneling service.

### 5. Register Webhook

Lastly, register the webhook to your Telegram-bot so any messages that your bot receives will be forwarded to Kumote via tunnel.

```bash
curl -X POST "https://api.telegram.org/bot<YOUR_BOT_TOKEN>/setWebhook" \
     -H "Content-Type: application/json" \
     -d '{"url": "https://your-tunnel.domain/telegram"}'
```

## 💬 Usage Examples

If you're reach this state, congratulations! 🎉

You've successfully setup Kumote and ready to rocks! Now try to send a message to your bot asking anything for your projects like you do with Claude Code CLI from your terminal.

## Notices

Below are some important notices that you should be aware of from this project.

### AI Agent

Initially, Kumote will use Claude Code CLI as the AI agent. The reason is I personally use Claude Code to interact with my projects for quick analysis and development. That's also the idea why I built this project.

You might not prefer to use it or simply don't have Claude Pro subscription. Therefore you can actually replace it with any other AI agent that you prefer with CLI interfaces. Currently what I've personally tried and works is Gemini CLI. It's totally free (with some usage limitation) and perform faster than Claude. But the quality is not as good as Claude.

There's also open source alternative called [OpenCode](https://github.com/sst/opencode) that local LLM for good privacy but I didn't try it yet.

### Privacy

IN PROGRESS

## Planned Features

- [ ] **Support Session (Claude Code Only)** - Support session for Claude Code CLI to keep the context of the conversation. This is useful for long conversations or when you want to keep the context of the conversation. (In development)
- [ ] **Support other CLI Agents** - Support for other CLI AI agents. Gemini CLI is in development. Suggestions or contributions are welcome!
- [ ] **Long Polling Interface** - Implement long polling interface for Telegram to avoid webhook setup.
- [ ] **Improve Project Detection** - Currently Kumote will determine the project by check words from message one by one. This is not efficient and can be improved by using more advanced techniques like fuzzy matching or regex.

## License

This project is licensed under the AGPL-v3.0 License. See the [LICENSE](LICENSE) file for details.

## Contributing

IN PROGRESS
