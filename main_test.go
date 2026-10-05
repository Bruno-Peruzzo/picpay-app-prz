package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleRoot(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handleRoot(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado status 200, obtido %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Hello") {
		t.Fatalf("corpo inesperado: %q", rec.Body.String())
	}
}

func TestHandleHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado status 200, obtido %d", rec.Code)
	}
}

func TestHandleReady(t *testing.T) {
	// Pronto: deve retornar 200.
	ready.Store(1)
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	handleReady(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ready=1: esperado 200, obtido %d", rec.Code)
	}

	// Não pronto: deve retornar 503.
	ready.Store(0)
	rec = httptest.NewRecorder()
	handleReady(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready=0: esperado 503, obtido %d", rec.Code)
	}
	ready.Store(1) // restaura
}

func TestRootNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/caminho-inexistente", nil)
	rec := httptest.NewRecorder()

	handleRoot(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperado 404 para caminho desconhecido, obtido %d", rec.Code)
	}
}
