package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Test default values
	cfg := Load()
	
	if cfg.Server.Port != "8081" {
		t.Errorf("Expected default port 8081, got %s", cfg.Server.Port)
	}
	
	if cfg.Crawler.UserAgent != "Mozilla/5.0 (compatible; WebCrawler/1.0)" {
		t.Errorf("Expected default user agent, got %s", cfg.Crawler.UserAgent)
	}
	
	if cfg.RateLimit.RequestsPerSecond != 5.0 {
		t.Errorf("Expected default rate limit 5.0, got %f", cfg.RateLimit.RequestsPerSecond)
	}
}

func TestLoadWithEnvVars(t *testing.T) {
	// Set environment variables
	os.Setenv("PORT", "9000")
	os.Setenv("MAX_CONCURRENT", "10")
	os.Setenv("REQUESTS_PER_SECOND", "2.5")
	
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("MAX_CONCURRENT")
		os.Unsetenv("REQUESTS_PER_SECOND")
	}()
	
	cfg := Load()
	
	if cfg.Server.Port != "9000" {
		t.Errorf("Expected port 9000 from env, got %s", cfg.Server.Port)
	}
	
	if cfg.RateLimit.MaxConcurrent != 10 {
		t.Errorf("Expected max concurrent 10 from env, got %d", cfg.RateLimit.MaxConcurrent)
	}
	
	if cfg.RateLimit.RequestsPerSecond != 2.5 {
		t.Errorf("Expected rate limit 2.5 from env, got %f", cfg.RateLimit.RequestsPerSecond)
	}
}

func TestGetDurationEnv(t *testing.T) {
	// Test with valid duration
	os.Setenv("TEST_DURATION", "5s")
	duration := getDurationEnv("TEST_DURATION", 10*time.Second)
	if duration != 5*time.Second {
		t.Errorf("Expected 5s, got %v", duration)
	}
	
	// Test with invalid duration (should return default)
	os.Setenv("TEST_DURATION", "invalid")
	duration = getDurationEnv("TEST_DURATION", 10*time.Second)
	if duration != 10*time.Second {
		t.Errorf("Expected default 10s for invalid duration, got %v", duration)
	}
	
	// Test with missing env var (should return default)
	os.Unsetenv("TEST_DURATION")
	duration = getDurationEnv("TEST_DURATION", 15*time.Second)
	if duration != 15*time.Second {
		t.Errorf("Expected default 15s for missing env var, got %v", duration)
	}
}