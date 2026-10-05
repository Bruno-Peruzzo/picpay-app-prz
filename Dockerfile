# ---- Estágio 1: build ----
# Compila um binário estático (CGO desabilitado) para rodar numa imagem mínima.
FROM golang:1.23-alpine AS build

WORKDIR /src

# Cache de dependências: copia go.mod/go.sum primeiro.
COPY go.mod go.sum* ./
RUN go mod download

# Copia o restante do código e compila.
COPY . .
# -trimpath e ldflags reduzem o tamanho e removem caminhos locais do binário.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/server .

# ---- Estágio 2: runtime ----
# distroless/static: sem shell, sem gerenciador de pacotes — superfície mínima.
# A tag :nonroot já roda como usuário não-root (uid 65532).
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /app/server /app/server

# Documenta a porta (a app lê PORT, default 8080).
EXPOSE 8080

USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
