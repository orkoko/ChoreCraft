package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/joho/godotenv"

	"ChoreCraft/internal/repository"
	"ChoreCraft/internal/service"
)

func TestGeminiIntegration(t *testing.T) {
	// Try to load the real .env file from the backend root directory
	_ = godotenv.Load("../../.env")

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" || apiKey == "your_gemini_api_key_here" {
		t.Skip("Skipping Gemini integration test: valid GEMINI_API_KEY not set")
	}

	repo := repository.New(testDbPool)
	svc := service.New(repo, apiKey, "", "", "")

	ctx := context.Background()

	// Ensure we use a unique title that isn't cached yet
	title := "Wash the elephant"

	// 1. Test synchronous resolution
	// Should just return the title since it's not in the DB initially
	resolvedTitle, needsAsync := svc.ResolveEmoji(ctx, title)
	if !needsAsync {
		t.Errorf("Expected needsAsync to be true for a new title")
	}
	if resolvedTitle != title {
		t.Errorf("Expected resolvedTitle to be unmodified initially, got: %s", resolvedTitle)
	}

	// 2. Test asynchronous resolution (calls Gemini)
	newTitle := svc.ResolveEmojiAsync(ctx, title)

	// We expect Gemini to prepend an emoji
	if newTitle == title {
		t.Fatalf("Expected Gemini to prepend an emoji to the title, but got original title: %s", newTitle)
	}

	// Check if the title starts with an emoji (should be longer and end with the original string)
	if !strings.HasSuffix(newTitle, title) {
		t.Errorf("Expected new title to end with original title, got: %s", newTitle)
	}

	if len(newTitle) <= len(title)+1 { // Should have an emoji + space prepended
		t.Errorf("Expected new title to contain an emoji, got: %s", newTitle)
	}

	// 3. Test caching behavior
	// Check if it saved to the database by resolving synchronously again
	cachedTitle, needsAsync2 := svc.ResolveEmoji(ctx, title)
	if needsAsync2 {
		t.Errorf("Expected needsAsync to be false after caching")
	}
	if cachedTitle != newTitle {
		t.Errorf("Expected cached title to match Gemini output %q, got: %q", newTitle, cachedTitle)
	}
}
