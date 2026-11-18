//go:build js && wasm

package httpclient

import (
	"net/http"
	"syscall/js"

	"github.com/syumai/workers-go/cloudflare/fetch"
)

// newClient wraps workers-go's fetch transport, which ignores the request
// context, so each fetch carries an AbortSignal tied to that context.
func newClient() *http.Client {
	return &http.Client{Transport: &abortTransport{prepare: prepareFetch}}
}

// prepareFetch routes one request through a binding whose fetch adds an
// AbortSignal to the RequestInit before calling the global fetch.
func prepareFetch() (http.RoundTripper, func()) {
	controller := js.Global().Get("AbortController").New()
	globalFetch := js.Global().Get("fetch")
	fn := js.FuncOf(func(_ js.Value, args []js.Value) any {
		init := args[1]
		init.Set("signal", controller.Get("signal"))
		return globalFetch.Invoke(args[0], init)
	})
	binding := js.Global().Get("Object").New()
	binding.Set("fetch", fn)

	// Workers rejects redirect mode "error", so fetch returns 3xx responses
	// and they are refused here to keep secret keys on the RevenueCat host.
	rt := fetch.NewClient(fetch.WithBinding(binding)).HTTPClient(fetch.RedirectModeManual).Transport
	send := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		defer fn.Release()
		res, err := rt.RoundTrip(req)
		if err != nil {
			return nil, err
		}
		if res.StatusCode >= 300 && res.StatusCode < 400 {
			controller.Call("abort")
			return nil, ErrRedirect
		}
		// Fetch Response.body is null on 204/304 responses; replace with NoBody
		// so reading resp.Body does not call getReader() on js.Null().
		if raw, ok := res.Body.(interface{ GetRawJSBody() js.Value }); ok {
			if jsBody := raw.GetRawJSBody(); jsBody.IsNull() || jsBody.IsUndefined() {
				res.Body = http.NoBody
			}
		}
		return res, nil
	})
	abort := func() { controller.Call("abort") }
	return send, abort
}
