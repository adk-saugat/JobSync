package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleRegisterInvalidJSON(t *testing.T) {
	srv := &Server{SyncSecret: "sync-secret"}
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`not-json`))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleRegisterMissingFields(t *testing.T) {
	srv := &Server{SyncSecret: "sync-secret"}
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
