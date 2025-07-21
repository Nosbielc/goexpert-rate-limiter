package ratelimiter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStorage é uma implementação mock da interface Storage
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) Increment(ctx context.Context, key string, window time.Duration) (int64, error) {
	args := m.Called(ctx, key, window)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockStorage) IsBlocked(ctx context.Context, key string) (bool, error) {
	args := m.Called(ctx, key)
	return args.Bool(0), args.Error(1)
}

func (m *MockStorage) Block(ctx context.Context, key string, duration time.Duration) error {
	args := m.Called(ctx, key, duration)
	return args.Error(0)
}

func (m *MockStorage) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestRateLimiter_CheckIP_AllowedRequests(t *testing.T) {
	mockStorage := &MockStorage{}
	config := Config{
		Requests:  5,
		Window:    time.Second,
		BlockTime: time.Minute,
	}

	rateLimiter := NewRateLimiter(mockStorage, config)

	ctx := context.Background()
	ip := "192.168.1.1"

	// Chamadas de armazenamento mockadas - cada solicitação deve ser permitida
	for i := 1; i <= 5; i++ {
		mockStorage.On("IsBlocked", ctx, "ip:"+ip).Return(false, nil).Once()
		mockStorage.On("Increment", ctx, "ip:"+ip, time.Second).Return(int64(i), nil).Once()
	}

	// As primeiras 5 solicitações devem ser permitidas
	for i := 0; i < 5; i++ {
		allowed, err := rateLimiter.CheckIP(ctx, ip)
		assert.NoError(t, err)
		assert.True(t, allowed)
	}

	mockStorage.AssertExpectations(t)
}

func TestRateLimiter_CheckIP_ExceedsLimit(t *testing.T) {
	mockStorage := &MockStorage{}
	config := Config{
		Requests:  2,
		Window:    time.Second,
		BlockTime: time.Minute,
	}

	rateLimiter := NewRateLimiter(mockStorage, config)

	ctx := context.Background()
	ip := "192.168.1.1"

	// Chamadas de armazenamento mockadas para limite excedido
	mockStorage.On("IsBlocked", ctx, "ip:"+ip).Return(false, nil).Once()
	mockStorage.On("Increment", ctx, "ip:"+ip, time.Second).Return(int64(3), nil).Once()
	mockStorage.On("Block", ctx, "ip:"+ip, time.Minute).Return(nil).Once()

	// A 3ª solicitação deve ser bloqueada (excede o limite de 2)
	allowed, err := rateLimiter.CheckIP(ctx, ip)
	assert.NoError(t, err)
	assert.False(t, allowed)

	mockStorage.AssertExpectations(t)
}

func TestRateLimiter_CheckIP_AlreadyBlocked(t *testing.T) {
	mockStorage := &MockStorage{}
	config := Config{
		Requests:  5,
		Window:    time.Second,
		BlockTime: time.Minute,
	}

	rateLimiter := NewRateLimiter(mockStorage, config)

	ctx := context.Background()
	ip := "192.168.1.1"

	// Chamadas de armazenamento mockadas para IP já bloqueado
	mockStorage.On("IsBlocked", ctx, "ip:"+ip).Return(true, nil).Once()
	// Nota: Quando já bloqueado, Increment não deve ser chamado

	// A solicitação deve ser bloqueada
	allowed, err := rateLimiter.CheckIP(ctx, ip)
	assert.NoError(t, err)
	assert.False(t, allowed)

	mockStorage.AssertExpectations(t)
}

func TestRateLimiter_CheckToken_ValidToken(t *testing.T) {
	mockStorage := &MockStorage{}
	ipConfig := Config{
		Requests:  5,
		Window:    time.Second,
		BlockTime: time.Minute,
	}

	rateLimiter := NewRateLimiter(mockStorage, ipConfig)

	// Adiciona configuração de token
	tokenConfig := Config{
		Requests:  10,
		Window:    time.Second,
		BlockTime: time.Minute * 2,
	}
	token := "abc123"
	rateLimiter.AddTokenConfig(token, tokenConfig)

	ctx := context.Background()

	// Chamadas de armazenamento mockadas
	mockStorage.On("IsBlocked", ctx, "token:"+token).Return(false, nil).Once()
	mockStorage.On("Increment", ctx, "token:"+token, time.Second).Return(int64(1), nil).Once()

	// Solicitação com token válido deve ser permitida
	allowed, err := rateLimiter.CheckToken(ctx, token)
	assert.NoError(t, err)
	assert.True(t, allowed)

	mockStorage.AssertExpectations(t)
}

func TestRateLimiter_CheckToken_InvalidToken(t *testing.T) {
	mockStorage := &MockStorage{}
	ipConfig := Config{
		Requests:  5,
		Window:    time.Second,
		BlockTime: time.Minute,
	}

	rateLimiter := NewRateLimiter(mockStorage, ipConfig)

	ctx := context.Background()
	token := "invalid_token"

	// Solicitação com token inválido deve ser permitida (reverte para limitação por IP)
	// Nenhuma chamada de armazenamento deve ser feita para token inválido
	allowed, err := rateLimiter.CheckToken(ctx, token)
	assert.NoError(t, err)
	assert.True(t, allowed)

	mockStorage.AssertExpectations(t)
}

func TestRateLimiter_CheckToken_ExceedsLimit(t *testing.T) {
	mockStorage := &MockStorage{}
	ipConfig := Config{
		Requests:  5,
		Window:    time.Second,
		BlockTime: time.Minute,
	}

	rateLimiter := NewRateLimiter(mockStorage, ipConfig)

	// Adiciona configuração de token com limite baixo
	tokenConfig := Config{
		Requests:  1,
		Window:    time.Second,
		BlockTime: time.Minute * 2,
	}
	token := "abc123"
	rateLimiter.AddTokenConfig(token, tokenConfig)

	ctx := context.Background()

	// Chamadas de armazenamento mockadas para limite excedido
	mockStorage.On("IsBlocked", ctx, "token:"+token).Return(false, nil).Once()
	mockStorage.On("Increment", ctx, "token:"+token, time.Second).Return(int64(2), nil).Once()
	mockStorage.On("Block", ctx, "token:"+token, time.Minute*2).Return(nil).Once()

	// A 2ª solicitação deve ser bloqueada (excede o limite de 1)
	allowed, err := rateLimiter.CheckToken(ctx, token)
	assert.NoError(t, err)
	assert.False(t, allowed)

	mockStorage.AssertExpectations(t)
}
