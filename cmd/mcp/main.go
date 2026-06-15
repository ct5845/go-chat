package main

import (
	"context"
	"ct-go-chat/src/infrastructure/agent"
	"ct-go-chat/src/infrastructure/agent/bedrock"
	"ct-go-chat/src/infrastructure/clock"
	"ct-go-chat/src/infrastructure/config"
	"ct-go-chat/src/infrastructure/conversation"
	"ct-go-chat/src/infrastructure/dice"
	"ct-go-chat/src/infrastructure/prompts"
	"ct-go-chat/src/infrastructure/tools"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type rollDiceArgs struct {
	Sides *int `json:"sides,omitempty" jsonschema:"number of sides per die"`
	Count *int `json:"count,omitempty" jsonschema:"number of dice to roll"`
}

type chatArgs struct {
	Request string `json:"request" jsonschema:"the natural-language request for the agent to handle"`
}

func main() {
	godotenv.Load()
	config.Load()
	// config.InitLogging() writes to stdout, which is the JSON-RPC channel for
	// the stdio transport — calling it would corrupt the protocol stream.
	run()
}

func run() {
	client, err := bedrock.NewClient(config.BedrockRegion, config.BedrockModelID)
	if err != nil {
		slog.Error("Failed to initialise Bedrock client", "error", err)
		os.Exit(1)
	}
	chatAgent := agent.New(client, prompts.System(), tools.All())

	store, err := conversation.NewStore(config.ConversationsDir, config.BedrockModelID)
	if err != nil {
		slog.Error("Failed to initialise conversation store", "error", err)
		os.Exit(1)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "ct-go-chat-mcp", Version: "v0.1.0"}, nil)

	rollDice(server)
	getTime(server)
	chat(server, chatAgent, store)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

func rollDice(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        dice.ToolName,
		Description: dice.ToolDescription,
	}, func(ctx context.Context, req *mcp.CallToolRequest, args rollDiceArgs) (*mcp.CallToolResult, any, error) {
		sides, count := 6, 1
		if args.Sides != nil {
			sides = *args.Sides
		}
		if args.Count != nil {
			count = *args.Count
		}

		text, err := dice.Roll(sides, count)
		if err != nil {
			return nil, nil, err
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
}

func getTime(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        clock.ToolName,
		Description: clock.ToolDescription,
	}, func(ctx context.Context, req *mcp.CallToolRequest, args any) (*mcp.CallToolResult, any, error) {
		res := clock.Now()

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: res}}}, nil, nil
	})
}

func chat(server *mcp.Server, chatAgent *agent.Agent, store *conversation.Store) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "chat",
		Description: "Hand a natural language request to the assistant agent, which may use its own tools to answer, and returns the agent's final response. Each call is independent and stateless — include any context the agent needs in the request itself; prior calls are not remembered.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args chatArgs) (*mcp.CallToolResult, any, error) {
		events := make(chan agent.Event)

		var exchange agent.Exchange
		var err error
		go func() {
			exchange, err = chatAgent.Run(ctx, nil, args.Request, events)
		}()

		for range events {
			// required to drain the events queue.
			// If we wanted to stream vs send final response back only, we'd do it here.
		}

		if err != nil {
			return nil, nil, err
		}
		exchange.Source = agent.SourceMCP

		conv := &conversation.Conversation{
			ID:        conversation.NewID(),
			Created:   time.Now(),
			Exchanges: []agent.Exchange{exchange},
		}

		if err := store.Save(conv); err != nil {
			slog.Error("failed to save mcp chat conversation", "error", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: exchange.Response}},
		}, nil, nil
	})
}
