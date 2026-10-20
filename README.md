# cryptotape

windows 11 screener for first-crossing Binance USDⓈ-M futures alerts. the Go market engine runs independently of the Wails UI. alert rules, settings, and the alert log live in SQLite.

## run and build

requires Go 1.26.1, Node.js 22+, [Wails 3 CLI](https://v3.wails.io/getting-started/installation/) and, for the installer, NSIS. Windows also needs WebView2; the installer includes its bootstrapper.

```sh
cd frontend
npm ci
npm run build
cd ..
go run .
```

```sh
wails3 task check
wails3 task installer:windows
```

the second command writes `build/bin/cryptotape.exe` and `build/bin/cryptotape-amd64-installer.exe`. cross-compiling on macOS produces Windows files but does not verify their behavior on Windows.

## signals

- subscribes to `@bookTicker` for trading USDT contracts above the configured 24-hour quote-volume threshold; volume comes from REST and `!miniTicker@arr`.
- evaluates bid/ask midpoint on every price change. each rule uses 100 ms extrema buckets for its configured rolling window. a symbol must fill the whole window after startup or a feed gap.
- emits the first crossing up or down per symbol and rule. a cooldown suppresses continuations and rebounds without producing a delayed signal at its end.
- persists an alert before the overlay, sound, or optional local terminal request. the terminal route is `POST http://localhost:5005/api/open/zenica/binance/F/{SYMBOL}` and never uses the Binance proxy.
- reconnects with backoff. stale quotes and lost connections reset the affected window; alerts resume only after warming again. the UI reads missed alerts by SQLite cursor.

settings are stored in the user's application config directory (`cryptotape/cryptotape.sqlite`), with imported sounds alongside it. the proxy accepts an HTTP CONNECT endpoint as `ip:port` and is applied to both Binance REST and WebSocket traffic. there is no direct-route fallback.

## checks

```sh
go test ./internal/... .
go test -tags integration ./internal/binance -run TestPublicFuturesFeed -count=1
cd frontend && npm run lint && npm run build
```

the integration test contacts the live Binance futures API. latency and resource targets need profiling under full market load on the intended Windows 11 machine. the five built-in sounds include the original `notification.mp3` for personal use; confirm distribution rights before publishing an installer.
