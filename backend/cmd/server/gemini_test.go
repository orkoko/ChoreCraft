package main

import (
	"context"
	"os"
	"path/filepath"
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

func TestSnapToFibonacci(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{4, 3},
		{6, 5},
		{10, 8},
		{12, 13},
		{25, 21},
		{40, 34},
	}

	for _, tc := range tests {
		got := service.SnapToFibonacci(tc.input)
		if got != tc.expected {
			t.Errorf("SnapToFibonacci(%d) = %d; want %d", tc.input, got, tc.expected)
		}
	}
}

func TestCalculateChoreCoins(t *testing.T) {
	tests := []struct {
		name           string
		usd            float64
		suggestedCoins int
		expected       int
	}{
		{"micro task <= $2", 1.5, 0, 1},
		{"quick task table wipe $4", 4.0, 0, 2},
		{"small chore full table clear & clean $7", 7.0, 0, 3},
		{"moderate chore mopping $15", 15.0, 0, 5},
		{"substantial chore $25", 25.0, 0, 8},
		{"large chore $45", 45.0, 0, 13},
		{"deep clean $70", 70.0, 0, 21},
		{"gemini suggested 2 coins", 4.0, 2, 2},
		{"gemini suggested 3 coins", 6.0, 3, 3},
		{"gemini suggested 4 snaps to 3", 8.0, 4, 3},
	}

	for _, tc := range tests {
		got := service.CalculateChoreCoins(tc.usd, tc.suggestedCoins)
		if got != tc.expected {
			t.Errorf("CalculateChoreCoins(%f, %d) [%s] = %d; want %d", tc.usd, tc.suggestedCoins, tc.name, got, tc.expected)
		}
	}
}

func TestCalculateRewardCoins(t *testing.T) {
	tests := []struct {
		name     string
		usd      float64
		expected int
	}{
		{"zero or negative fallback", 0.0, 10},
		{"micro treat $1", 1.0, 2},
		{"snack $2.50", 2.5, 6}, // round(2.5) = 3 -> 3 * 2 = 6
		{"candy bar $3", 3.0, 6},
		{"ice cream $5", 5.0, 10},
		{"cinema ticket $10", 10.0, 20},
		{"small toy $12", 12.0, 20}, // ((12+2)/5)*5 = 10 * 2 = 20
		{"board game $15", 15.0, 30},
		{"pizza $20", 20.0, 40},
		{"lego set $25", 25.0, 50},
		{"headphones $40", 40.0, 80},
		{"video game $60", 60.0, 120},
		{"smartwatch $100", 100.0, 200},
		{"bicycle $250", 250.0, 500},
		{"ps5 console $500", 500.0, 1000},
		{"gaming laptop $1000", 1000.0, 2000},
	}

	for _, tc := range tests {
		got := service.CalculateRewardCoins(tc.usd)
		if got != tc.expected {
			t.Errorf("CalculateRewardCoins(%f) [%s] = %d; want %d", tc.usd, tc.name, got, tc.expected)
		}
	}
}

func TestDeleteUploadedPhoto(t *testing.T) {
	_ = os.MkdirAll("./uploads", 0755)
	dummyFile := filepath.Join("./uploads", "test_delete_sample.jpg")
	if err := os.WriteFile(dummyFile, []byte("fake image content"), 0644); err != nil {
		t.Fatalf("failed to create dummy file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(dummyFile); os.IsNotExist(err) {
		t.Fatalf("dummy file should exist before delete")
	}

	// Test deletion with /uploads/ path
	service.DeleteUploadedPhoto("/uploads/test_delete_sample.jpg")

	// Verify file is gone
	if _, err := os.Stat(dummyFile); !os.IsNotExist(err) {
		t.Errorf("expected dummy file to be deleted, but still exists")
	}

	// Test that non-local or empty URLs do not error/panic
	service.DeleteUploadedPhoto("")
	service.DeleteUploadedPhoto("https://external.domain.com/photo.jpg")
}

