package executor

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/msjurset/runbook/internal/runbook"
)

func TestHTTPExecutorGET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("X-Custom", "test-value")
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	e := &HTTPExecutor{
		Step: &runbook.HTTPStep{
			Method: "GET",
			URL:    srv.URL + "/health",
		},
	}

	var stdout bytes.Buffer
	result, err := e.Execute(context.Background(), map[string]string{}, &stdout, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
	if result.Body != `{"status":"ok"}` {
		t.Errorf("Body = %q, want %q", result.Body, `{"status":"ok"}`)
	}
	if result.Headers.Get("X-Custom") != "test-value" {
		t.Errorf("missing X-Custom header")
	}
	if result.Output() != `{"status":"ok"}` {
		t.Errorf("Output() = %q, want body content", result.Output())
	}
}

func TestHTTPExecutorPOST(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(201)
		w.Write([]byte("created"))
	}))
	defer srv.Close()

	e := &HTTPExecutor{
		Step: &runbook.HTTPStep{
			Method:  "POST",
			URL:     srv.URL + "/items",
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    `{"name":"{{.item}}"}`,
		},
	}

	vars := map[string]string{"item": "widget"}
	result, err := e.Execute(context.Background(), vars, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StatusCode != 201 {
		t.Errorf("StatusCode = %d, want 201", result.StatusCode)
	}
}

func TestHTTPExecutorDefaultMethod(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected default GET, got %s", r.Method)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	e := &HTTPExecutor{
		Step: &runbook.HTTPStep{
			URL: srv.URL,
		},
	}

	_, err := e.Execute(context.Background(), map[string]string{}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPExecutor4xxError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()

	e := &HTTPExecutor{
		Step: &runbook.HTTPStep{
			Method: "GET",
			URL:    srv.URL + "/missing",
		},
	}

	result, err := e.Execute(context.Background(), map[string]string{}, nil, nil)
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if result.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", result.StatusCode)
	}
	if result.Body != "not found" {
		t.Errorf("Body = %q, want %q", result.Body, "not found")
	}
}

func TestHTTPExecutor5xxError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	e := &HTTPExecutor{
		Step: &runbook.HTTPStep{
			Method: "GET",
			URL:    srv.URL,
		},
	}

	_, err := e.Execute(context.Background(), map[string]string{}, nil, nil)
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}

func TestHTTPExecutorTemplateExpansion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/status" {
			t.Errorf("path = %q, want /api/v2/status", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret123" {
			t.Errorf("Authorization = %q, want 'Bearer secret123'", r.Header.Get("Authorization"))
		}
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	e := &HTTPExecutor{
		Step: &runbook.HTTPStep{
			Method:  "GET",
			URL:     srv.URL + "/api/{{.version}}/status",
			Headers: map[string]string{"Authorization": "Bearer {{.token}}"},
		},
	}

	vars := map[string]string{"version": "v2", "token": "secret123"}
	_, err := e.Execute(context.Background(), vars, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPExecutorCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	e := &HTTPExecutor{
		Step: &runbook.HTTPStep{
			Method: "GET",
			URL:    srv.URL,
		},
	}

	_, err := e.Execute(ctx, map[string]string{}, nil, nil)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}
