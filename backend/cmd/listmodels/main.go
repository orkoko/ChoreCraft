package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/joho/godotenv"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

func main() {
	_ = godotenv.Load()
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY not set")
	}
	fmt.Printf("API Key prefix: %s...\n", apiKey[:10])

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Println("\n=== Available Models (supporting generateContent) ===")
	iter := client.ListModels(ctx)
	for {
		m, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("Error listing models: %v", err)
			break
		}
		for _, method := range m.SupportedGenerationMethods {
			if strings.Contains(method, "generateContent") {
				fmt.Printf("  %-45s  methods: %v\n", m.Name, m.SupportedGenerationMethods)
				break
			}
		}
	}
}
