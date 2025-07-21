package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cleibson/goexpert-rate-limiter/internal/ratelimiter"
	"github.com/joho/godotenv"
)

// Config armazena toda a configuração da aplicação
type Config struct {
	Redis  RedisConfig
	IP     ratelimiter.Config
	Tokens map[string]ratelimiter.Config
}

// RedisConfig armazena a configuração de conexão Redis
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// Load carrega configuração a partir de variáveis de ambiente
func Load() (*Config, error) {
	// Carrega arquivo .env se existir
	_ = godotenv.Load()

	config := &Config{
		Tokens: make(map[string]ratelimiter.Config),
	}

	// Carrega configuração Redis
	config.Redis.Addr = getEnv("REDIS_ADDR", "localhost:6379")
	config.Redis.Password = getEnv("REDIS_PASSWORD", "")
	config.Redis.DB = getEnvAsInt("REDIS_DB", 0)

	// Carrega configuração de limitação de IP
	ipRequests := getEnvAsInt64("RATE_LIMIT_IP_REQUESTS", 10)
	ipWindow, err := time.ParseDuration(getEnv("RATE_LIMIT_IP_WINDOW", "1s"))
	if err != nil {
		return nil, fmt.Errorf("duração inválida da janela de IP: %w", err)
	}
	ipBlockTime, err := time.ParseDuration(getEnv("RATE_LIMIT_IP_BLOCK_TIME", "5m"))
	if err != nil {
		return nil, fmt.Errorf("duração inválida do tempo de bloqueio de IP: %w", err)
	}

	config.IP = ratelimiter.Config{
		Requests:  ipRequests,
		Window:    ipWindow,
		BlockTime: ipBlockTime,
	}

	// Carrega configurações de tokens
	err = config.loadTokenConfigs()
	if err != nil {
		return nil, fmt.Errorf("falha ao carregar configurações de tokens: %w", err)
	}

	return config, nil
}

// loadTokenConfigs carrega configurações específicas de tokens a partir de variáveis de ambiente
func (c *Config) loadTokenConfigs() error {
	// Procura por variáveis de ambiente com padrão RATE_LIMIT_TOKEN_<TOKEN>_*
	for _, env := range os.Environ() {
		parts := strings.Split(env, "=")
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		if !strings.HasPrefix(key, "RATE_LIMIT_TOKEN_") || !strings.HasSuffix(key, "_REQUESTS") {
			continue
		}

		// Extrai nome do token
		tokenPart := strings.TrimPrefix(key, "RATE_LIMIT_TOKEN_")
		tokenPart = strings.TrimSuffix(tokenPart, "_REQUESTS")

		if tokenPart == "" {
			continue
		}

		// Carrega configuração do token
		requests := getEnvAsInt64(fmt.Sprintf("RATE_LIMIT_TOKEN_%s_REQUESTS", tokenPart), 0)
		if requests == 0 {
			continue
		}

		windowStr := getEnv(fmt.Sprintf("RATE_LIMIT_TOKEN_%s_WINDOW", tokenPart), "1s")
		window, err := time.ParseDuration(windowStr)
		if err != nil {
			return fmt.Errorf("duração inválida da janela para token %s: %w", tokenPart, err)
		}

		blockTimeStr := getEnv(fmt.Sprintf("RATE_LIMIT_TOKEN_%s_BLOCK_TIME", tokenPart), "5m")
		blockTime, err := time.ParseDuration(blockTimeStr)
		if err != nil {
			return fmt.Errorf("duração inválida do tempo de bloqueio para token %s: %w", tokenPart, err)
		}

		c.Tokens[tokenPart] = ratelimiter.Config{
			Requests:  requests,
			Window:    window,
			BlockTime: blockTime,
		}
	}

	return nil
}

// getEnv obtém uma variável de ambiente com um valor padrão
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt obtém uma variável de ambiente como um inteiro com um valor padrão
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}

// getEnvAsInt64 obtém uma variável de ambiente como um int64 com um valor padrão
func getEnvAsInt64(key string, defaultValue int64) int64 {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.ParseInt(valueStr, 10, 64)
	if err != nil {
		return defaultValue
	}

	return value
}
