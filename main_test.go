package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDecodeBase64UTF8(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"standard base64", "aGVsbG8gd29ybGQ=", "hello world", false},
		{"url-safe base64", "PDw_Pz4-", "<<??>>", false},
		{"utf8 chinese", "5L2g5aW9", "你好", false},
		{"surrounding whitespace", "  aGVsbG8=\n", "hello", false},
		{"invalid base64", "!!!not base64!!!", "", true},
		{"decoded not utf8", "gIE=", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeBase64UTF8(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFetch(t *testing.T) {
	t.Run("success returns body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "aGVsbG8=")
		}))
		defer srv.Close()
		got, err := fetch(srv.Client(), srv.URL, 3, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "aGVsbG8=" {
			t.Fatalf("got %q, want %q", got, "aGVsbG8=")
		}
	})

	t.Run("retries on 5xx then succeeds", func(t *testing.T) {
		var calls int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if atomic.AddInt32(&calls, 1) <= 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			fmt.Fprint(w, "b2s=")
		}))
		defer srv.Close()
		got, err := fetch(srv.Client(), srv.URL, 3, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "b2s=" {
			t.Fatalf("got %q, want %q", got, "b2s=")
		}
		if n := atomic.LoadInt32(&calls); n != 3 {
			t.Fatalf("server called %d times, want 3", n)
		}
	})

	t.Run("4xx fails without retry", func(t *testing.T) {
		var calls int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&calls, 1)
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()
		_, err := fetch(srv.Client(), srv.URL, 3, 0)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if n := atomic.LoadInt32(&calls); n != 1 {
			t.Fatalf("server called %d times, want 1", n)
		}
	})

	t.Run("exhausted retries returns error", func(t *testing.T) {
		var calls int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&calls, 1)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()
		_, err := fetch(srv.Client(), srv.URL, 2, 0)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if n := atomic.LoadInt32(&calls); n != 3 {
			t.Fatalf("server called %d times, want 3", n)
		}
	})

	t.Run("timeout is retried then fails", func(t *testing.T) {
		var calls int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&calls, 1)
			time.Sleep(200 * time.Millisecond)
		}))
		defer srv.Close()
		client := &http.Client{Timeout: 50 * time.Millisecond}
		_, err := fetch(client, srv.URL, 1, 0)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if n := atomic.LoadInt32(&calls); n != 2 {
			t.Fatalf("server called %d times, want 2", n)
		}
	})
}
