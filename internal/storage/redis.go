package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisStorage implementa a interface Storage usando Redis
type RedisStorage struct {
	client *redis.Client
}

// NewRedisStorage cria uma nova instância de armazenamento Redis
func NewRedisStorage(addr, password string, db int) *RedisStorage {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	return &RedisStorage{
		client: rdb,
	}
}

// Increment incrementa o contador para uma chave específica e retorna a contagem atual
func (r *RedisStorage) Increment(ctx context.Context, key string, window time.Duration) (int64, error) {
	pipe := r.client.Pipeline()

	// Incrementa o contador
	incrCmd := pipe.Incr(ctx, key)

	// Define expiração se esta for a primeira incrementação
	pipe.Expire(ctx, key, window)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("falha ao incrementar contador: %w", err)
	}

	return incrCmd.Val(), nil
}

// IsBlocked verifica se uma chave está atualmente bloqueada
func (r *RedisStorage) IsBlocked(ctx context.Context, key string) (bool, error) {
	blockedKey := fmt.Sprintf("blocked:%s", key)

	result, err := r.client.Exists(ctx, blockedKey).Result()
	if err != nil {
		return false, fmt.Errorf("falha ao verificar se a chave está bloqueada: %w", err)
	}

	return result > 0, nil
}

// Block bloqueia uma chave pela duração especificada
func (r *RedisStorage) Block(ctx context.Context, key string, duration time.Duration) error {
	blockedKey := fmt.Sprintf("blocked:%s", key)

	err := r.client.Set(ctx, blockedKey, "1", duration).Err()
	if err != nil {
		return fmt.Errorf("falha ao bloquear chave: %w", err)
	}

	return nil
}

// Close fecha a conexão Redis
func (r *RedisStorage) Close() error {
	return r.client.Close()
}
