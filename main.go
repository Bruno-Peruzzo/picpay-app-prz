// Aplicação de demonstração do desafio PicPay EKS.
//
// Expõe os endpoints exigidos:
//   GET /         -> "hello" (página simples)
//   GET /health   -> liveness probe (sempre 200 enquanto o processo vive)
//   GET /ready    -> readiness probe (200 quando pronto; 503 durante o shutdown)
//   GET /metrics  -> métricas no formato Prometheus (client_golang)
//
// Instrumentação Prometheus:
//   - http_requests_total{method,path,status}      (counter)       — métricas HTTP padrão
//   - http_request_duration_seconds{method,path}   (histogram)     — latência por rota
//   - picpay_greetings_total                        (counter custom) — métrica custom do desafio
//
// A métrica custom (picpay_greetings_total) incrementa a cada acesso em "/",
// servindo para demonstrar no Grafana um número de negócio reagindo a tráfego real.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Métricas HTTP padrão (dimensões: método, rota e status).
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total de requisições HTTP processadas, por método, rota e status.",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duração das requisições HTTP em segundos, por método e rota.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Métrica custom do desafio: conta quantas saudações foram servidas em "/".
	greetingsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "picpay_greetings_total",
			Help: "Total de saudações servidas pelo endpoint raiz (métrica custom do desafio).",
		},
	)
)

// ready controla o estado de readiness. Começa pronto (1) e vira 0 no shutdown,
// para que o Kubernetes pare de enviar tráfego antes de encerrar o processo.
var ready atomic.Int32

// statusRecorder captura o status code escrito, para alimentar as métricas.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// instrument é um middleware que registra contador e histograma por requisição.
func instrument(path string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next(rec, r)

		elapsed := time.Since(start).Seconds()
		httpRequestDuration.WithLabelValues(r.Method, path).Observe(elapsed)
		httpRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(rec.status)).Inc()
	}
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	// Garante que "/" não capture qualquer caminho desconhecido.
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	greetingsTotal.Inc()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("Hello from PicPay EKS Challenge! Versao 2\n"))
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	// Liveness: o processo está vivo e respondendo.
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func handleReady(w http.ResponseWriter, _ *http.Request) {
	// Readiness: só aceita tráfego quando ready==1.
	if ready.Load() == 1 {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready\n"))
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte("not ready\n"))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	port := getenv("PORT", "8080")

	mux := http.NewServeMux()
	mux.HandleFunc("/", instrument("/", handleRoot))
	mux.HandleFunc("/health", instrument("/health", handleHealth))
	mux.HandleFunc("/ready", instrument("/ready", handleReady))
	// /metrics não é instrumentado para não poluir as próprias métricas.
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ready.Store(1)

	// Sobe o servidor numa goroutine para permitir shutdown gracioso.
	go func() {
		log.Printf("servidor ouvindo em :%s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("erro no servidor: %v", err)
		}
	}()

	// Aguarda sinal de término (SIGTERM do Kubernetes, Ctrl+C local).
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("sinal de término recebido; iniciando shutdown gracioso")

	// Marca como "not ready" para o Kubernetes drenar o tráfego antes de encerrar.
	ready.Store(0)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("erro no shutdown: %v", err)
	}
	log.Println("servidor encerrado")
}
