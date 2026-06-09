package config

import (
	"os"
)

var Port string
var AppEnv string
var BedrockRegion string
var BedrockModelID string
var ConversationsDir string

func Load() {
	Port = getEnvOr("PORT", "8080")
	AppEnv = getEnvOr("APP_ENV", "dev")
	BedrockRegion = getEnvOr("AWS_REGION", "us-east-1")
	BedrockModelID = getEnvOr("BEDROCK_MODEL_ID", "us.anthropic.claude-haiku-4-5-20251001-v1:0")
	ConversationsDir = getEnvOr("CONVERSATIONS_DIR", "data/conversations")
}

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
