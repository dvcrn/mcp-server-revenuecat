//go:build !(js && wasm)

package httpclient

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestNewRefusesRedirects(t *testing.T) {
	var redirected atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirected.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/leak", http.StatusFound)
	}))
	defer origin.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, origin.URL+"/v2/projects", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer sk_dummy_projectA")

	_, err = New().Do(req)
	if !errors.Is(err, ErrRedirect) {
		t.Fatalf("err = %v, want ErrRedirect", err)
	}
	if redirected.Load() {
		t.Fatal("client followed redirect")
	}
}
