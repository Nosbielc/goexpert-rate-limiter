package storage

import (
	"context"
	"time"
)

// Storage define a interface para estratégias de armazenamento do rate limiter
type Storage interface {
	// Increment incrementa o contador para uma chave específica e retorna a contagem atual
	Increment(ctx context.Context, key string, window time.Duration) (int64, error)

	// IsBlocked verifica se uma chave está atualmente bloqueada
	IsBlocked(ctx context.Context, key string) (bool, error)

	// Block bloqueia uma chave pela duração especificada
	Block(ctx context.Context, key string, duration time.Duration) error

	// Close fecha a conexão de armazenamento
	Close() error
}
