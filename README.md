# 🤖 Kumote - Remote Work Telegram Assistant

**KumoTe** (雲手) - Japanese for "Cloud Hand" - Your remote development companion that extends your coding capabilities through the cloud.

A secure Telegram bot that provides remote access to your local development environment through Claude Code CLI. Control your projects, execute commands, and manage your development workflow from anywhere.

## ✨ Features

- 🔐 **Secure Remote Access** - Access your development machine through encrypted Telegram messages
- 🤖 **AI-Powered Commands** - Leverage Claude Code CLI for intelligent code analysis and execution
- 📁 **Smart Project Discovery** - Automatically discovers and indexes your development projects
- 🎤 **Voice Commands** - Process audio messages for hands-free operation
- 🚀 **Real-time Responses** - Instant webhook-based communication
- 📊 **Built-in Monitoring** - Health checks and metrics collection
- 🛡️ **Enterprise Security** - User whitelisting, rate limiting, and webhook verification

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

- Go 1.21+
- Telegram Bot Token ([Get one from @BotFather](https://t.me/botfather))
- Claude Code CLI installed
- Cloudflare account (for tunnel)

### 1. Clone & Setup

```bash
git clone https://github.com/yourusername/kumote.git
cd kumote

# Copy environment template
cp .env.example .env
```

### 2. Configure Environment

```bash
# .env
TELEGRAM_BOT_TOKEN=your_bot_token_here
TELEGRAM_WEBHOOK_SECRET=your_webhook_secret
ALLOWED_USER_IDS=123456789,987654321
DEVELOPMENT_PATH=/home/user/Development
CLAUDE_CODE_PATH=/usr/local/bin/claude-code
```

### 3. Run with Docker

```bash
# Start the application
docker-compose up --build

# View logs
docker-compose logs -f telegram-assistant
```

### 4. Setup Cloudflare Tunnel

```bash
# Install cloudflared
# Then run:
cloudflared tunnel --url localhost:3000
```

### 5. Register Webhook

```bash
curl -X POST "https://api.telegram.org/bot<YOUR_BOT_TOKEN>/setWebhook" \
     -H "Content-Type: application/json" \
     -d '{"url": "https://your-tunnel-url.trycloudflare.com/telegram-webhook"}'
```

## 💬 Usage Examples

### Basic Commands

```
📱 User: "show taqwa main.go"
🤖 Bot: [Returns main.go content from TaqwaBoard project]

📱 User: "git status all"
🤖 Bot: [Shows git status for all projects]

📱 User: "update dependencies taqwa"
🤖 Bot: [Runs dependency update in TaqwaBoard]
```

### Project Shortcuts

```yaml
# Configure in scanner-config.yaml
shortcuts:
  taqwa: TaqwaBoard
  car: CarLogbook
  jda: Junior-Dev-Acceleration
```

### Voice Commands

- Send voice messages for hands-free operation
- Automatic speech-to-text conversion
- Same functionality as text commands

## 🛠️ Development

### Local Development

```bash
# Install dependencies
go mod download

# Run tests
go test ./... -v

# Run with hot reload
go run main.go
```

### Project Structure

```
kumote/
├── main.go                           # Application entry point
├── internal/
│   ├── assistant/
│   │   ├── core/                    # Business logic
│   │   └── adapters/                # External integrations
│   └── presentation/
│       ├── handlers/                # HTTP handlers
│       ├── middleware/              # HTTP middleware
│       └── server/                  # Server setup
├── config/
│   └── scanner-config.yaml         # Project discovery config
└── docker-compose.yml              # Deployment setup
```

### Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run benchmarks
go test ./... -bench=. -benchmem
```

## 📊 Monitoring

### Health Check

```bash
curl http://localhost:3000/health
curl http://localhost:3000/health?detailed=true
```

### Metrics

```bash
curl http://localhost:3000/metrics
```

### Logs

```bash
# View application logs
docker-compose logs -f telegram-assistant

# View specific service logs
docker-compose logs cloudflared
```

## 🔒 Security

### Authentication

- **User Whitelist**: Only specified Telegram user IDs can access
- **Webhook Verification**: Telegram webhook signatures validated
- **Rate Limiting**: Configurable per-user request limits

### Network Security

- **Cloudflare Tunnel**: No direct port exposure
- **HTTPS Only**: All communication encrypted
- **No Data Persistence**: Messages not stored locally

### Command Safety

- **Safe Command Whitelist**: Dangerous operations blocked
- **Path Validation**: Access limited to Development directory
- **Input Sanitization**: All user input validated

## 🌐 Deployment

### Production Deployment

```bash
# Build optimized image
docker build -t kumote:latest .

# Deploy with monitoring
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

### Environment Variables

| Variable                | Description                | Default         |
| ----------------------- | -------------------------- | --------------- |
| `TELEGRAM_BOT_TOKEN`    | Bot token from @BotFather  | **Required**    |
| `ALLOWED_USER_IDS`      | Comma-separated user IDs   | **Required**    |
| `DEVELOPMENT_PATH`      | Path to projects directory | `~/Development` |
| `SERVER_PORT`           | HTTP server port           | `3000`          |
| `LOG_LEVEL`             | Logging level              | `info`          |
| `RATE_LIMIT_PER_MINUTE` | Rate limit per user        | `10`            |

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- Write table-driven tests for all new functionality
- Follow Go best practices and conventions
- Update documentation for any API changes
- Ensure all tests pass before submitting PR

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Telegram Bot API](https://core.telegram.org/bots/api) for webhook functionality
- [Claude Code CLI](https://docs.anthropic.com) for AI-powered code execution
- [Cloudflare Tunnels](https://www.cloudflare.com/products/tunnel/) for secure networking

---

**Kumote** - Extending your development reach through the cloud ☁️✋

Telegram Webhooks:

- https://core.telegram.org/bots/webhooks
- https://core.telegram.org/bots/api#setwebhook
