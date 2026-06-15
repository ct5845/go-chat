# ct-go-chat

A streaming AI chat application built on Go, HTMX, Alpine.js, and TailwindCSS — backed by AWS Bedrock. Also ships a Model Context Protocol (MCP) server that exposes the same tools and agent to external clients such as Claude Desktop.

Forked from [ct-go-web-starter](https://github.com/ct5845/ct-go-web-starter), which provides the base infrastructure (routing, live reload, static assets, compression, graceful shutdown).

## Features

- **Streaming chat** — responses stream token-by-token via SSE, with cancel support
- **Agent with tools** — the chat agent can call tools mid-response (currently `roll_dice` and `get_time`)
- **MCP server** — a second binary (`cmd/mcp`) exposes the tools and the agent over the Model Context Protocol (see [MCP Server](#mcp-server))
- **Conversation history** — conversations are persisted and reloadable by ID, with per-exchange origin (`web` or `mcp`)
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
│   ├── web/           # Web app entrypoint (starts the HTTP server)
│   ├── mcp/           # MCP server entrypoint (stdio transport)
│   └── copyassets/    # Build tool: copies static assets and JS deps to tmp/
├── src/
│   ├── features/
│   │   ├── chat/          # Chat page, SSE stream handler, input component
│   │   │   ├── chatinput/ # Message input UI
│   │   │   ├── chatstream/ # POST /chat/stream — SSE handler
│   │   │   └── history/   # Conversation history sidebar
│   │   ├── home/          # Home/landing page
│   │   └── nav/           # Bottom navigation tabs
│   ├── components/    # Shared UI building blocks
│   └── infrastructure/ # Platform/runtime concerns with no HTTP surface
│       ├── agent/         # LLM-in-a-loop with tools
│       ├── tools/         # Agent tool definitions (roll_dice, get_time)
│       ├── dice/          # Dice-roll logic + tool metadata (shared)
│       ├── clock/         # Current-time logic + tool metadata (shared)
│       ├── conversation/  # Conversation store (load/save) + usage/cost totals
│       └── prompts/       # The agent's system prompt
├── .air.windows.toml  # Live reload config (Windows)
├── .air.linux.toml    # Live reload config (Linux/macOS)
└── package.json       # Frontend dependencies
```

Tool logic (`dice`, `clock`) lives in surface-free `infrastructure` packages so it can be shared by both the in-process agent tools (`infrastructure/tools`) and the MCP server (`cmd/mcp`) without duplication.

### Available Commands

- `make web` — Start development server with live reload
- `make build` — Build CSS, copy assets, and compile the web binary
- `make build-mcp` — Compile the MCP server binary to `build/mcp`
- `make docker` — Build the production Docker image (web app only)

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

> The Docker image builds the **web app** only. The MCP server is a stdio program launched by its client (see below) and is not containerised.

## MCP Server

`cmd/mcp` is a [Model Context Protocol](https://modelcontextprotocol.io) server that exposes this project's capabilities to external MCP clients (e.g. Claude Desktop). It communicates over **stdio** — the client launches the binary and talks to it over the child process's stdin/stdout.

### Tools

| Tool | Description | Cost |
| --- | --- | --- |
| `roll_dice` | Roll one or more dice. Deterministic; no LLM. | Consumer's tokens |
| `get_time` | Current date/time in the server's timezone. Deterministic; no LLM. | Consumer's tokens |
| `chat` | Hand a natural-language request to the chat agent, which may use its own tools to answer. Each call is independent and stateless — the caller must include all needed context in the request. | **This project's tokens** (runs Bedrock) |

The deterministic tools (`roll_dice`, `get_time`) share their logic with the web app's agent — the same `dice.Roll` / `clock.Now` backs both. The `chat` tool runs the full agent (`agent.Run` with `prompts.System()` and `tools.All()`), so a single `chat` call may itself trigger the agent's *internal* tool use. Cancelling the tool call cancels the agent run.

Each `chat` invocation persists a conversation to `CONVERSATIONS_DIR` with `source: "mcp"` on its exchange(s), so MCP usage and cost appear alongside web chats in the same store.

### Build

```bash
make build-mcp        # produces build/mcp (or build/mcp.exe on Windows)
```

### Connecting Claude Desktop

Add an entry to Claude Desktop's config (`%APPDATA%\Claude\claude_desktop_config.json` on Windows, `~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "ct-go-chat": {
      "command": "/absolute/path/to/build/mcp",
      "env": {
        "AWS_REGION": "us-east-1",
        "BEDROCK_MODEL_ID": "us.anthropic.claude-haiku-4-5-20251001-v1:0",
        "CONVERSATIONS_DIR": "/absolute/path/to/data/conversations"
      }
    }
  }
}
```

Notes:
- **Use absolute paths.** Claude Desktop spawns the binary with an unpredictable working directory, so `CONVERSATIONS_DIR` must be absolute or conversation files will scatter.
- **AWS credentials** are resolved via the standard AWS credential chain (`~/.aws/credentials`, `~/.aws/config`, env vars). No credentials need to go in the config if you use shared `~/.aws` files.
- **Never call `config.InitLogging()` in the MCP binary** — it writes to stdout, which is the JSON-RPC channel. Logging goes to stderr (visible in Claude Desktop's MCP logs under `%APPDATA%\Claude\logs\`).

Restart Claude Desktop after editing the config. Rebuild with `make build-mcp` and restart whenever the MCP code changes.

## License

MIT
