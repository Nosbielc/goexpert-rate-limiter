package ratelimiter

import (
	"context"
	"fmt"
	"time"

	"github.com/cleibson/goexpert-rate-limiter/internal/storage"
)

// Config armazena a configuração do rate limiter
type Config struct {
	Requests  int64
	Window    time.Duration
	BlockTime time.Duration
}

// RateLimiter gerencia a lógica de limitação de taxa
type RateLimiter struct {
	storage  storage.Storage
	ipConfig Config
	tokens   map[string]Config
}

// NewRateLimiter cria uma nova instância do rate limiter
func NewRateLimiter(storage storage.Storage, ipConfig Config) *RateLimiter {
	return &RateLimiter{
		storage:  storage,
		ipConfig: ipConfig,
		tokens:   make(map[string]Config),
	}
}

// AddTokenConfig adiciona uma configuração de token
func (rl *RateLimiter) AddTokenConfig(token string, config Config) {
	rl.tokens[token] = config
}

// CheckIP verifica se um endereço IP tem permissão para fazer uma requisição
func (rl *RateLimiter) CheckIP(ctx context.Context, ip string) (bool, error) {
	key := fmt.Sprintf("ip:%s", ip)
	return rl.checkLimit(ctx, key, rl.ipConfig)
}

// CheckToken verifica se um token tem permissão para fazer uma requisição
func (rl *RateLimiter) CheckToken(ctx context.Context, token string) (bool, error) {
	config, exists := rl.tokens[token]
	if !exists {
		// Se a configuração do token não existe, volta para limitação baseada em IP
		return true, nil
	}

	key := fmt.Sprintf("token:%s", token)
	return rl.checkLimit(ctx, key, config)
}

// checkLimit executa a verificação de limitação de taxa
func (rl *RateLimiter) checkLimit(ctx context.Context, key string, config Config) (bool, error) {
	// Primeiro verifica se a chave está atualmente bloqueada
	blocked, err := rl.storage.IsBlocked(ctx, key)
	if err != nil {
		return false, fmt.Errorf("falha ao verificar se está bloqueado: %w", err)
	}

	if blocked {
		return false, nil
	}

	// Incrementa o contador e obtém a contagem atual
	count, err := rl.storage.Increment(ctx, key, config.Window)
	if err != nil {
		return false, fmt.Errorf("falha ao incrementar contador: %w", err)
	}

	// Verifica se o limite foi excedido
	if count > config.Requests {
		// Bloqueia a chave pela duração especificada
		err = rl.storage.Block(ctx, key, config.BlockTime)
		if err != nil {
			return false, fmt.Errorf("falha ao bloquear chave: %w", err)
		}
		return false, nil
	}

	return true, nil
}
