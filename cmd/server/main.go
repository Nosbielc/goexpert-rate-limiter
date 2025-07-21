package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cleibson/goexpert-rate-limiter/internal/config"
	"github.com/cleibson/goexpert-rate-limiter/internal/middleware"
	"github.com/cleibson/goexpert-rate-limiter/internal/ratelimiter"
	"github.com/cleibson/goexpert-rate-limiter/internal/storage"
)

func main() {
	// Carrega configuração
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Falha ao carregar configuração: %v", err)
	}

	// Inicializa armazenamento Redis
	redisStorage := storage.NewRedisStorage(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	defer redisStorage.Close()

	// Testa conexão Redis
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Inicializa rate limiter
	rateLimiter := ratelimiter.NewRateLimiter(redisStorage, cfg.IP)

	// Adiciona configurações de tokens
	for token, tokenConfig := range cfg.Tokens {
		rateLimiter.AddTokenConfig(token, tokenConfig)
		log.Printf("Configuração de token adicionada para '%s': %d req/%s, tempo de bloqueio: %s",
			token, tokenConfig.Requests, tokenConfig.Window, tokenConfig.BlockTime)
	}

	// Inicializa middleware
	rateLimiterMiddleware := middleware.NewRateLimiterMiddleware(rateLimiter)

	// Configura rotas
	mux := http.NewServeMux()

	// Endpoint de verificação de saúde
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	// Endpoint de teste
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "Request successful!", "timestamp": "` + time.Now().Format(time.RFC3339) + `"}`))
	})

	// Encapsula com middleware de rate limiter
	handler := rateLimiterMiddleware.Handler(mux)

	// Configura servidor
	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	// Inicia servidor em uma goroutine
	go func() {
		log.Printf("Iniciando servidor na porta 8080...")
		log.Printf("Limite de Taxa por IP: %d req/%s, tempo de bloqueio: %s",
			cfg.IP.Requests, cfg.IP.Window, cfg.IP.BlockTime)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Falha ao iniciar servidor: %v", err)
		}
	}()

	// Aguarda sinal de interrupção para encerrar o servidor graciosamente
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Encerrando servidor...")

	// Encerramento gracioso
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Servidor forçado a encerrar: %v", err)
	}

	log.Println("Servidor encerrado")
}
