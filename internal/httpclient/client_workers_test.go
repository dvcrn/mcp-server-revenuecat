//go:build js && wasm

package httpclient

import (
	"errors"
	"io"
	"net/http"
	"syscall/js"
	"testing"
)

func TestWorkersClientHandles204NullBodyAndRefusesRedirect(t *testing.T) {
	origFetch := js.Global().Get("fetch")
	t.Cleanup(func() { js.Global().Set("fetch", origFetch) })

	var nextStatus int
	mockFetch := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		init := js.Global().Get("Object").New()
		init.Set("status", nextStatus)
		if nextStatus == http.StatusFound {
			headers := js.Global().Get("Headers").New()
			headers.Call("set", "Location", "https://evil.example")
			init.Set("headers", headers)
		}
		resp := js.Global().Get("Response").New(js.Null(), init)
		return js.Global().Get("Promise").Call("resolve", resp)
	})
	defer mockFetch.Release()
	js.Global().Set("fetch", mockFetch)

	client := New()

	nextStatus = http.StatusNoContent
	req204, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, "https://api.revenuecat.com/v2/projects/proj1ab2c3d4/products/prod_dummy", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req204)
	if err != nil {
		t.Fatalf("204 Do error: %v", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil || len(data) != 0 {
		t.Fatalf("204 ReadAll = (%q, %v), want empty", data, err)
	}

	nextStatus = http.StatusFound
	req302, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.revenuecat.com/v2/projects", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Do(req302); !errors.Is(err, ErrRedirect) {
		t.Fatalf("302 Do err = %v, want ErrRedirect", err)
	}
}
