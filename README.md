# ct-go-chat

A streaming AI chat application built on Go, HTMX, Alpine.js, and TailwindCSS — backed by AWS Bedrock.

Forked from [ct-go-web-starter](https://github.com/ct5845/ct-go-web-starter), which provides the base infrastructure (routing, live reload, static assets, compression, graceful shutdown).

## Features

- **Streaming chat** — responses stream token-by-token via SSE, with cancel support
- **Conversation history** — conversations are persisted and reloadable by ID
- **AWS Bedrock** — LLM backend via the Bedrock streaming API
- **HTMX + Alpine.js** — reactive UI without a JS build step
- **TailwindCSS** — utility-first styling
- **Live Reload** — Air integration for development hot reloading
- **Feature-Based Architecture** — vertical slices under `src/features/`

## Quick Start

### Prerequisites

- Go 1.26 or later
- Node.js 24 or later
- [Air](https://github.com/air-verse/air) for live reload (`go install github.com/air-verse/air@latest`)
- AWS credentials with Bedrock access (via environment, `~/.aws`, or IAM role)

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/ct5845/ct-go-chat
   cd ct-go-chat
   ```

2. Install dependencies:
   ```bash
   npm install
   go mod tidy
   ```

3. Copy `.env.example` to `.env` and set your Bedrock region and model:
   ```bash
   cp .env.example .env
   ```

4. Run the development server:
   ```bash
   make web
   ```

The application will be available at `http://localhost:8080` (or the port set in `PORT`).

> `make web` detects the OS automatically — it uses `.air.windows.toml` on Windows and `.air.linux.toml` on Linux/macOS.

## Development

### Project Structure

```
├── cmd/
│   ├── web/           # Main entrypoint (starts the server)
│   └── copyassets/    # Build tool: copies static assets and JS deps to tmp/
├── src/
│   ├── features/
│   │   ├── chat/          # Chat page, SSE stream handler, input component
│   │   │   ├── chatinput/ # Message input UI
│   │   │   ├── chatstream/ # POST /chat/stream — SSE handler
│   │   │   └── history/   # Conversation history sidebar
│   │   ├── conversation/  # Conversation store (load/save)
│   │   ├── home/          # Home/landing page
│   │   └── nav/           # Bottom navigation tabs
│   ├── components/    # Shared UI building blocks
│   ├── infrastructure/ # Config, compression, file serving, LLM client
│   └── app.go         # Application setup and routing
├── .air.windows.toml  # Live reload config (Windows)
├── .air.linux.toml    # Live reload config (Linux/macOS)
└── package.json       # Frontend dependencies
```

### Available Commands

- `make web` — Start development server with live reload
- `make build` — Build CSS, copy assets, and compile production binary
- `make docker` — Build the production Docker image

## Production

Build and run:

```bash
make build
./build/web
```

### Docker

```bash
make docker
docker run -p 8080:8080 -e AWS_REGION=us-east-1 ct-go-chat
```

The image uses a three-stage build (Node → Go → distroless) for a minimal runtime with no shell or package manager. Configuration is via environment variables only — no `.env` file at runtime.

## License

MIT
