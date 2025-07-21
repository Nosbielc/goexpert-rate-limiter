package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/cleibson/goexpert-rate-limiter/internal/ratelimiter"
)

// RateLimiterMiddleware encapsula a funcionalidade do rate limiter como um middleware HTTP
type RateLimiterMiddleware struct {
	rateLimiter *ratelimiter.RateLimiter
}

// NewRateLimiterMiddleware cria um novo middleware de rate limiter
func NewRateLimiterMiddleware(rateLimiter *ratelimiter.RateLimiter) *RateLimiterMiddleware {
	return &RateLimiterMiddleware{
		rateLimiter: rateLimiter,
	}
}

// Handler retorna o handler do middleware HTTP
func (m *RateLimiterMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()

		// Extrai o endereço IP
		ip := m.getClientIP(r)

		// Extrai a chave da API do header
		apiKey := r.Header.Get("API_KEY")

		var allowed bool
		var err error

		// Verifica token primeiro (tem precedência sobre IP)
		if apiKey != "" {
			allowed, err = m.rateLimiter.CheckToken(ctx, apiKey)
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		} else {
			// Volta para limitação baseada em IP
			allowed, err = m.rateLimiter.CheckIP(ctx, ip)
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		if !allowed {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error": "you have reached the maximum number of requests or actions allowed within a certain time frame"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getClientIP extrai o endereço IP do cliente a partir da requisição
func (m *RateLimiterMiddleware) getClientIP(r *http.Request) string {
	// Verifica primeiro o header X-Forwarded-For
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		// Pega o primeiro IP se houver múltiplos
		ips := strings.Split(xForwardedFor, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Verifica o header X-Real-IP
	xRealIP := r.Header.Get("X-Real-IP")
	if xRealIP != "" {
		return xRealIP
	}

	// Volta para RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}
