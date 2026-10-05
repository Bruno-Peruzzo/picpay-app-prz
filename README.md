# picpay-app

Aplicação Go de demonstração do desafio PicPay EKS. Serve uma saudação simples e
expõe métricas no formato Prometheus, para demonstrar observabilidade end-to-end
(Prometheus + Grafana) rodando em Amazon EKS.

Este repositório contém **apenas a aplicação**. A infraestrutura (Terraform/EKS),
o Helm chart e os manifests GitOps (ArgoCD) vivem no repositório de infraestrutura
[`picpay-challenge-prz`](https://github.com/Bruno-Peruzzo/picpay-challenge-prz).
O CI deste repo builda a imagem e publica no **Amazon ECR**; o repo de infra a
consome pela tag da imagem via GitOps.

## Endpoints

| Rota | Método | Descrição |
|---|---|---|
| `/` | GET | Saudação (incrementa a métrica custom `picpay_greetings_total`) |
| `/health` | GET | Liveness probe — 200 enquanto o processo vive |
| `/ready` | GET | Readiness probe — 200 quando pronto; 503 durante o shutdown |
| `/metrics` | GET | Métricas no formato Prometheus |

## Métricas

- `picpay_greetings_total` (counter) — métrica custom; conta saudações servidas em `/`.
- `http_requests_total{method,path,status}` (counter) — requisições HTTP.
- `http_request_duration_seconds{method,path}` (histogram) — latência por rota.

## Configuração

| Variável | Default | Descrição |
|---|---|---|
| `PORT` | `8080` | Porta HTTP do servidor |

## Rodar localmente

Requisitos: Go 1.23+.

```bash
go mod tidy          # garante o go.sum
go test ./...        # roda a suíte de testes
go run .             # sobe o servidor em :8080
```

Em outro terminal:

```bash
curl localhost:8080/
curl localhost:8080/health
curl localhost:8080/ready
curl localhost:8080/metrics | grep picpay_greetings_total
```

A cada hit em `/`, o contador `picpay_greetings_total` sobe — visível no `/metrics`.

## Rodar com Docker

Imagem multi-stage (build em `golang:alpine`, runtime em `distroless/static`
não-root — superfície mínima).

```bash
docker build -t picpay-app:dev .
docker run --rm -p 8080:8080 picpay-app:dev
```

Depois, os mesmos `curl` acima.

## Shutdown gracioso

Ao receber `SIGTERM`/`SIGINT`, o servidor marca `/ready` como 503 (para o
Kubernetes drenar o tráfego) e encerra as conexões em andamento com timeout.

<!-- ci: trigger rollback-window test 1791243519 -->
