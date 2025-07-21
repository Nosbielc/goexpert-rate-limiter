package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cleibson/goexpert-rate-limiter/internal/ratelimiter"
	"github.com/stretchr/testify/assert"
)

// InMemoryStorage é um armazenamento simples em memória para testes
type InMemoryStorage struct {
	counters map[string]countData
	blocked  map[string]time.Time
}

type countData struct {
	count    int64
	expireAt time.Time
}

func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		counters: make(map[string]countData),
		blocked:  make(map[string]time.Time),
	}
}

func (s *InMemoryStorage) Increment(ctx context.Context, key string, window time.Duration) (int64, error) {
	now := time.Now()

	if data, exists := s.counters[key]; exists && now.Before(data.expireAt) {
		data.count++
		s.counters[key] = data
		return data.count, nil
	}

	// Reset counter with new expiration
	s.counters[key] = countData{
		count:    1,
		expireAt: now.Add(window),
	}
	return 1, nil
}

func (s *InMemoryStorage) IsBlocked(ctx context.Context, key string) (bool, error) {
	if blockedUntil, exists := s.blocked[key]; exists {
		return time.Now().Before(blockedUntil), nil
	}
	return false, nil
}

func (s *InMemoryStorage) Block(ctx context.Context, key string, duration time.Duration) error {
	s.blocked[key] = time.Now().Add(duration)
	return nil
}

func (s *InMemoryStorage) Close() error {
	return nil
}

func TestRateLimiterMiddleware_IPLimiting(t *testing.T) {
	storage := NewInMemoryStorage()
	config := ratelimiter.Config{
		Requests:  3,
		Window:    time.Second,
		BlockTime: time.Minute,
	}

	rateLimiter := ratelimiter.NewRateLimiter(storage, config)
	middleware := NewRateLimiterMiddleware(rateLimiter)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Testa solicitações bem-sucedidas dentro do limite
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "OK", recorder.Body.String())
	}

	// Testa solicitação que excede o limite
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusTooManyRequests, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "you have reached the maximum number of requests")
}

func TestRateLimiterMiddleware_TokenLimiting(t *testing.T) {
	storage := NewInMemoryStorage()
	ipConfig := ratelimiter.Config{
		Requests:  2,
		Window:    time.Second,
		BlockTime: time.Minute,
	}

	rateLimiter := ratelimiter.NewRateLimiter(storage, ipConfig)

	// Adiciona token com limite maior
	tokenConfig := ratelimiter.Config{
		Requests:  5,
		Window:    time.Second,
		BlockTime: time.Minute,
	}
	rateLimiter.AddTokenConfig("abc123", tokenConfig)

	middleware := NewRateLimiterMiddleware(rateLimiter)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Testa solicitações com token (deve permitir 5 solicitações)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("API_KEY", "abc123")
		req.RemoteAddr = "192.168.1.1:12345"

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "OK", recorder.Body.String())
	}

	// Testa solicitação que excede o limite do token
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("API_KEY", "abc123")
	req.RemoteAddr = "192.168.1.1:12345"

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusTooManyRequests, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "you have reached the maximum number of requests")
}

func TestRateLimiterMiddleware_TokenOverridesIP(t *testing.T) {
	storage := NewInMemoryStorage()
	ipConfig := ratelimiter.Config{
		Requests:  1, // Limite de IP muito baixo
		Window:    time.Second,
		BlockTime: time.Minute,
	}

	rateLimiter := ratelimiter.NewRateLimiter(storage, ipConfig)

	// Adiciona token com limite maior
	tokenConfig := ratelimiter.Config{
		Requests:  3,
		Window:    time.Second,
		BlockTime: time.Minute,
	}
	rateLimiter.AddTokenConfig("abc123", tokenConfig)

	middleware := NewRateLimiterMiddleware(rateLimiter)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Faz uma solicitação sem token (deve ser limitada pela configuração de IP)
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusOK, recorder.Code)

	// Segunda solicitação sem token deve ser bloqueada
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusTooManyRequests, recorder.Code)

	// Reinicia armazenamento para novo teste
	storage = NewInMemoryStorage()
	rateLimiter = ratelimiter.NewRateLimiter(storage, ipConfig)
	rateLimiter.AddTokenConfig("abc123", tokenConfig)
	middleware = NewRateLimiterMiddleware(rateLimiter)
	handler = middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Faz solicitações com token (deve permitir 3 solicitações apesar do limite baixo de IP)
	for i := 0; i < 3; i++ {
		req = httptest.NewRequest("GET", "/", nil)
		req.Header.Set("API_KEY", "abc123")
		req.RemoteAddr = "192.168.1.1:12345"

		recorder = httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		assert.Equal(t, http.StatusOK, recorder.Code)
	}
}

func TestRateLimiterMiddleware_GetClientIP(t *testing.T) {
	middleware := &RateLimiterMiddleware{}

	tests := []struct {
		name         string
		setupRequest func(*http.Request)
		expectedIP   string
	}{
		{
			name: "Header X-Forwarded-For",
			setupRequest: func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "203.0.113.1, 198.51.100.1")
			},
			expectedIP: "203.0.113.1",
		},
		{
			name: "Header X-Real-IP",
			setupRequest: func(r *http.Request) {
				r.Header.Set("X-Real-IP", "203.0.113.2")
			},
			expectedIP: "203.0.113.2",
		},
		{
			name: "Fallback RemoteAddr",
			setupRequest: func(r *http.Request) {
				r.RemoteAddr = "192.168.1.1:12345"
			},
			expectedIP: "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			tt.setupRequest(req)

			ip := middleware.getClientIP(req)
			assert.Equal(t, tt.expectedIP, ip)
		})
	}
}
