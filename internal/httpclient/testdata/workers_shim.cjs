// Stub Cloudflare Workers binding context expected by syumai/workers-go/internal/jsutil.init
// when running GOOS=js GOARCH=wasm tests under Node's go_js_wasm_exec.
globalThis.context ??= { binding: {} };
