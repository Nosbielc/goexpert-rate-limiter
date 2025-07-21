# Rate Limiter em Go

Este projeto implementa um rate limiter robusto em Go que pode ser configurado para limitar o número máximo de requisições por segundo com base em endereço IP ou token de acesso.

## Características

- **Limitação por IP**: Controla requisições baseadas no endereço IP do cliente
- **Limitação por Token**: Permite diferentes limites para tokens específicos (via header `API_KEY`)
- **Precedência de Token**: Configurações de token sobrepõem as configurações de IP
- **Armazenamento Redis**: Utiliza Redis para persistência das informações de limite
- **Estratégia Plugável**: Interface de storage permite trocar facilmente o Redis por outro mecanismo
- **Middleware HTTP**: Integração fácil como middleware em servidores web
- **Configuração via Ambiente**: Configuração flexível através de variáveis de ambiente ou arquivo `.env`

## Arquitetura

O projeto segue uma arquitetura limpa com separação de responsabilidades:

```
├── cmd/server/           # Aplicação principal
├── internal/
│   ├── config/          # Carregamento de configurações
│   ├── middleware/      # Middleware HTTP para rate limiting
│   ├── ratelimiter/     # Lógica principal do rate limiter
│   └── storage/         # Interface e implementações de storage
├── docker-compose.yml   # Configuração Docker com Redis
├── Dockerfile          # Container da aplicação
└── .env               # Configurações de ambiente
```

## Configuração

### Variáveis de Ambiente

O rate limiter é configurado através de variáveis de ambiente:

#### Configurações do Redis
```bash
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
```

#### Configurações de IP
```bash
RATE_LIMIT_IP_REQUESTS=10      # Máximo de requisições por janela de tempo
RATE_LIMIT_IP_WINDOW=1s        # Janela de tempo (1s, 1m, 1h, etc.)
RATE_LIMIT_IP_BLOCK_TIME=5m    # Tempo de bloqueio após exceder o limite
```

#### Configurações de Token
```bash
# Para o token "abc123"
RATE_LIMIT_TOKEN_abc123_REQUESTS=100
RATE_LIMIT_TOKEN_abc123_WINDOW=1s
RATE_LIMIT_TOKEN_abc123_BLOCK_TIME=2m

# Para o token "xyz789"
RATE_LIMIT_TOKEN_xyz789_REQUESTS=50
RATE_LIMIT_TOKEN_xyz789_WINDOW=1s
RATE_LIMIT_TOKEN_xyz789_BLOCK_TIME=3m
```

## Como Executar

### Com Docker Compose (Recomendado)

1. Clone o repositório
2. Execute com Docker Compose:

```bash
docker-compose up --build
```

Isso iniciará tanto o Redis quanto a aplicação na porta 8080.

### Localmente

1. Inicie o Redis:
```bash
docker run -d -p 6379:6379 redis:7-alpine
```

2. Execute a aplicação:
```bash
go mod download
go run cmd/server/main.go
```

## Testando o Rate Limiter

### Teste de Limitação por IP

```bash
# Faça várias requisições rapidamente
for i in {1..15}; do
  curl -w "Status: %{http_code}\n" http://localhost:8080/
done
```

### Teste de Limitação por Token

```bash
# Teste com token válido
for i in {1..105}; do
  curl -H "API_KEY: abc123" -w "Status: %{http_code}\n" http://localhost:8080/
done

# Teste com token inválido (usa limitação por IP)
for i in {1..15}; do
  curl -H "API_KEY: invalid_token" -w "Status: %{http_code}\n" http://localhost:8080/
done
```

### Respostas Esperadas

**Requisição Bem-sucedida (Status 200):**
```json
{
  "message": "Request successful!", 
  "timestamp": "2025-07-21T10:30:00Z"
}
```

**Limite Excedido (Status 429):**
```json
{
  "error": "you have reached the maximum number of requests or actions allowed within a certain time frame"
}
```

## Funcionamento

### Fluxo de Decisão

1. **Extração de Identificador**: O middleware extrai o IP do cliente e verifica se há um token `API_KEY` no header
2. **Verificação de Token**: Se um token válido for fornecido, usa as configurações do token
3. **Fallback para IP**: Se não há token ou token inválido, usa as configurações de IP
4. **Verificação de Bloqueio**: Verifica se o identificador está atualmente bloqueado
5. **Contagem de Requisições**: Incrementa o contador para a janela de tempo atual
6. **Verificação de Limite**: Se exceder o limite, bloqueia por um período determinado
7. **Resposta**: Permite ou nega a requisição

### Algoritmo de Rate Limiting

O rate limiter usa um algoritmo de **janela deslizante** implementado com Redis:

- **Contador por Janela**: Cada IP/token tem um contador que expira após a janela de tempo
- **Bloqueio Temporal**: Quando o limite é excedido, o identificador é bloqueado por um período configurável
- **Expiração Automática**: Contadores e bloqueios expiram automaticamente

## Testes

### Executar Testes Unitários

```bash
go test ./internal/ratelimiter -v
go test ./internal/middleware -v
```

### Testes de Carga

Para testar sob alta carga, você pode usar ferramentas como `hey` ou `apache bench`:

```bash
# Instalar hey
go install github.com/rakyll/hey@latest

# Teste de carga
hey -n 1000 -c 10 -H "API_KEY: abc123" http://localhost:8080/
```

## Estrutura de Storage

### Interface Storage

```go
type Storage interface {
    Increment(ctx context.Context, key string, window time.Duration) (int64, error)
    IsBlocked(ctx context.Context, key string) (bool, error)
    Block(ctx context.Context, key string, duration time.Duration) error
    Close() error
}
```

### Implementação Redis

A implementação Redis usa:
- **Pipelines** para operações atômicas
- **Expiração automática** de chaves
- **Prefixos** para organizar diferentes tipos de dados (`ip:`, `token:`, `blocked:`)

## Monitoramento

### Health Check

```bash
curl http://localhost:8080/health
```

### Logs

A aplicação registra:
- Configurações carregadas na inicialização
- Tokens configurados com seus limites
- Status do servidor

## Considerações de Produção

1. **Persistência Redis**: Configure Redis com persistência em produção
2. **Clustering**: Para alta disponibilidade, use Redis Cluster
3. **Monitoramento**: Monitore métricas do Redis e da aplicação
4. **Configuração de Rede**: Configure adequadamente headers de proxy (`X-Forwarded-For`)
5. **Logs**: Implemente logging estruturado para auditoria

## Extensibilidade

### Adicionando Novos Storages

Implemente a interface `Storage` para adicionar novos mecanismos de persistência:

```go
type MyStorage struct{}

func (s *MyStorage) Increment(ctx context.Context, key string, window time.Duration) (int64, error) {
    // Sua implementação
}

func (s *MyStorage) IsBlocked(ctx context.Context, key string) (bool, error) {
    // Sua implementação
}

func (s *MyStorage) Block(ctx context.Context, key string, duration time.Duration) error {
    // Sua implementação
}

func (s *MyStorage) Close() error {
    // Sua implementação
}
```

### Configuração Dinâmica de Tokens

Para adicionar novos tokens dinamicamente, adicione variáveis de ambiente seguindo o padrão:

```bash
RATE_LIMIT_TOKEN_<TOKEN_NAME>_REQUESTS=<NUMBER>
RATE_LIMIT_TOKEN_<TOKEN_NAME>_WINDOW=<DURATION>
RATE_LIMIT_TOKEN_<TOKEN_NAME>_BLOCK_TIME=<DURATION>
```

## Exemplos de Uso

### Integração em Servidor Existente

```go
package main

import (
    "github.com/cleibson/goexpert-rate-limiter/internal/config"
    "github.com/cleibson/goexpert-rate-limiter/internal/middleware"
    "github.com/cleibson/goexpert-rate-limiter/internal/ratelimiter"
    "github.com/cleibson/goexpert-rate-limiter/internal/storage"
)

func main() {
    cfg, _ := config.Load()
    storage := storage.NewRedisStorage(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
    rateLimiter := ratelimiter.NewRateLimiter(storage, cfg.IP)
    
    for token, config := range cfg.Tokens {
        rateLimiter.AddTokenConfig(token, config)
    }
    
    middleware := middleware.NewRateLimiterMiddleware(rateLimiter)
    
    // Use middleware.Handler(yourHandler) em seu servidor
}
```
