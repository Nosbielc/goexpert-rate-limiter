#!/bin/bash

# Script para testar o Rate Limiter
# Este script demonstra como testar o rate limiter em diferentes cenários

echo "=== Rate Limiter Test Script ==="
echo ""

# Verificar se o servidor está rodando
if ! curl -s http://localhost:8080/health > /dev/null; then
    echo "❌ Servidor não está rodando na porta 8080"
    echo "Execute primeiro: docker-compose up ou go run cmd/server/main.go"
    exit 1
fi

echo "✅ Servidor está rodando"
echo ""

# Teste 1: Limitação por IP (deve permitir 10 requests por segundo)
echo "🔍 Teste 1: Limitação por IP (limite: 10 req/s)"
echo "Fazendo 15 requisições rapidamente..."

success_count=0
blocked_count=0

for i in {1..15}; do
    response=$(curl -s -w "%{http_code}" -o /dev/null http://localhost:8080/)
    if [ "$response" -eq 200 ]; then
        success_count=$((success_count + 1))
        echo "  Requisição $i: ✅ 200 OK"
    elif [ "$response" -eq 429 ]; then
        blocked_count=$((blocked_count + 1))
        echo "  Requisição $i: ❌ 429 Rate Limited"
    else
        echo "  Requisição $i: ⚠️  $response"
    fi
    sleep 0.1
done

echo "Resultado: $success_count sucessos, $blocked_count bloqueadas"
echo ""

# Aguardar um pouco para reset
echo "⏳ Aguardando 2 segundos..."
sleep 2

# Teste 2: Limitação por Token (deve permitir 100 requests por segundo para abc123)
echo "🔍 Teste 2: Limitação por Token 'abc123' (limite: 100 req/s)"
echo "Fazendo 105 requisições rapidamente com token..."

success_count=0
blocked_count=0

for i in {1..105}; do
    response=$(curl -s -w "%{http_code}" -o /dev/null -H "API_KEY: abc123" http://localhost:8080/)
    if [ "$response" -eq 200 ]; then
        success_count=$((success_count + 1))
        if [ $((i % 20)) -eq 0 ]; then
            echo "  Requisições 1-$i: ✅ $success_count sucessos"
        fi
    elif [ "$response" -eq 429 ]; then
        blocked_count=$((blocked_count + 1))
        echo "  Requisição $i: ❌ 429 Rate Limited"
    else
        echo "  Requisição $i: ⚠️  $response"
    fi
    sleep 0.01
done

echo "Resultado: $success_count sucessos, $blocked_count bloqueadas"
echo ""

# Teste 3: Token inválido (deve usar limitação por IP)
echo "🔍 Teste 3: Token inválido 'invalid' (deve usar limite IP: 10 req/s)"
echo "Fazendo 15 requisições com token inválido..."

success_count=0
blocked_count=0

for i in {1..15}; do
    response=$(curl -s -w "%{http_code}" -o /dev/null -H "API_KEY: invalid" http://localhost:8080/)
    if [ "$response" -eq 200 ]; then
        success_count=$((success_count + 1))
        echo "  Requisição $i: ✅ 200 OK"
    elif [ "$response" -eq 429 ]; then
        blocked_count=$((blocked_count + 1))
        echo "  Requisição $i: ❌ 429 Rate Limited"
    else
        echo "  Requisição $i: ⚠️  $response"
    fi
    sleep 0.1
done

echo "Resultado: $success_count sucessos, $blocked_count bloqueadas"
echo ""

# Teste 4: Verificar precedência de token sobre IP
echo "🔍 Teste 4: Verificando precedência de token sobre IP"
echo "Usando token 'xyz789' (limite: 50 req/s) vs IP (limite: 10 req/s)"

success_count=0
blocked_count=0

for i in {1..55}; do
    response=$(curl -s -w "%{http_code}" -o /dev/null -H "API_KEY: xyz789" http://localhost:8080/)
    if [ "$response" -eq 200 ]; then
        success_count=$((success_count + 1))
        if [ $((i % 10)) -eq 0 ]; then
            echo "  Requisições 1-$i: ✅ $success_count sucessos"
        fi
    elif [ "$response" -eq 429 ]; then
        blocked_count=$((blocked_count + 1))
        echo "  Requisição $i: ❌ 429 Rate Limited"
    else
        echo "  Requisição $i: ⚠️  $response"
    fi
    sleep 0.02
done

echo "Resultado: $success_count sucessos, $blocked_count bloqueadas"
echo ""

echo "🎉 Testes concluídos!"
echo ""
echo "📊 Resumo esperado:"
echo "  - Teste 1 (IP): ~10 sucessos, ~5 bloqueadas"
echo "  - Teste 2 (Token abc123): ~100 sucessos, ~5 bloqueadas"  
echo "  - Teste 3 (Token inválido): ~10 sucessos, ~5 bloqueadas"
echo "  - Teste 4 (Token xyz789): ~50 sucessos, ~5 bloqueadas"
echo ""
echo "💡 Nota: Os números podem variar devido ao timing das requisições"
