package env

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Load() error {
	environment := os.Getenv("APP_ENV")
	if environment == "" {
		environment = os.Getenv("NODE_ENV")
	}
	if environment == "" {
		environment = "development"
	}
	if environment != "test" && environment != "development" && environment != "production" {
		return fmt.Errorf("unsupported APP_ENV %q; expected test, development, or production", environment)
	}

	fileName := ".env." + environment
	candidates := []string{
		filepath.Join("config", "env", fileName),
		filepath.Join("..", "..", "config", "env", fileName),
		filepath.Join("..", "..", "..", "config", "env", fileName),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return loadFile(candidate)
		}
	}

	return nil
}

func loadFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open environment file %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return fmt.Errorf("invalid environment entry at %s:%d", path, lineNumber)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, "\"'")
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("set environment variable %s: %w", key, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read environment file %s: %w", path, err)
	}
	return nil
}
