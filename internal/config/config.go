package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the terminal endpoint server.
type Config struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	MaxSessions     int
	SessionTTL      time.Duration
	MaxOutputLines  int
	MaxExecTimeout  time.Duration
	DefaultShell    string
	BufferSize      int
	FlushInterval   time.Duration
	MaxFlushBytes   int
	MaxExecOutputKB int
}

// Default returns a Config populated with sensible defaults.
func Default() *Config {
	return &Config{
		Host:            getEnv("HOST", "127.0.0.1"),
		Port:            getEnvInt("PORT", 8080),
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    60 * time.Second,
		MaxSessions:     getEnvInt("MAX_SESSIONS", 50),
		SessionTTL:      getEnvDuration("SESSION_TTL", 5*time.Minute),
		MaxOutputLines:  getEnvInt("MAX_OUTPUT_LINES", 1000),
		MaxExecTimeout:  getEnvDuration("MAX_EXEC_TIMEOUT", 120*time.Second),
		DefaultShell:    getEnv("SHELL", "/bin/bash"),
		BufferSize:      getEnvInt("BUFFER_SIZE", 4096),
		FlushInterval:   getEnvDuration("FLUSH_INTERVAL", 50*time.Millisecond),
		MaxFlushBytes:   getEnvInt("MAX_FLUSH_BYTES", 16384),
		MaxExecOutputKB: getEnvInt("MAX_EXEC_OUTPUT_KB", 100),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
