# cryptotape

windows 11 screener for first-crossing Binance USDⓈ-M futures alerts. the Go market engine runs independently of the Wails UI. alert rules, settings, and the alert log live in SQLite.

## run and build

requires Go 1.26.1, Node.js 22+, and [Wails 3 CLI](https://v3.wails.io/getting-started/installation/). Windows also needs the WebView2 runtime.

```sh
cd frontend
npm ci
npm run build
cd ..
go run .
```

on macos use `wails3 task run:mac` or `wails3 task build:mac`. transparent notification windows require Wails’ `private_mac_apis` build tag; the default macos build deliberately leaves the webview background opaque. this build is for local use outside the mac app store.

```sh
wails3 task check
wails3 task build:windows
```

the second command writes `build/bin/cryptotape.exe`. all frontend assets and built-in sounds are embedded; no files need to sit beside the executable. windows requires WebView2 runtime. cross-compiling on macos does not verify native windows behavior.

## releases and updates

pushing a version tag such as `v0.1.3` runs `.github/workflows/release.yml`: frontend/backend checks, a windows build and publication of `cryptotape.exe`. the tag must match the versions in `frontend/package.json` and `build/windows/info.json`.

download the exe and run it. to update, close cryptotape and replace the old exe. settings, history and imported sounds live separately in `%APPDATA%\cryptotape` and survive replacement or relocation of the executable for the same windows user. there is no installer or in-app automatic updater.

## signals

- subscribes to `@bookTicker` for trading USDT contracts above the lowest per-rule 24-hour quote-volume threshold; volume comes from REST and `!miniTicker@arr`. every five minutes, REST reconciles turnover and trading contracts; new listings join existing websocket shards without restarting other symbols.
- evaluates bid/ask midpoint on every price change. each rule uses 100 ms extrema buckets for its configured rolling window. the window is a maximum lookback: a crossing in partial fresh history alerts immediately.
- emits the first crossing up or down per symbol and rule. a cooldown suppresses continuations and rebounds without producing a delayed signal at its end.
- persists an alert before the overlay, sound, or optional local terminal request. the terminal route is `POST http://localhost:5005/api/open/zenica/binance/F/{SYMBOL}` and never uses the Binance proxy.
- keeps one native sound playing: each new alert stops the previous sound. every alert with auto-open enabled sends its own terminal request.
- reconnects after 1, 2, 4 and 8 seconds, then waits for manual reconnect. a session with fresh data that stays connected for a minute resets the retry budget. stale quotes and lost connections reset the affected window; alerts use the available fresh history immediately, without waiting for a full window. movement across a connection gap is never inferred. the UI reads missed alerts by SQLite cursor.

each rule has a badge color, a volume from 0 to 100%, its own turnover floor, and independent Vataga/Tiger auto-open choices. rules are always active; delete a rule from its editor to remove it. custom mp3/wav import lives in the sound selector.

the main window contains connection status and the virtualized alert feed. rule management lives in settings. bursts are batched per frame without discarding history; notifications retain the latest three cards. reading older alerts keeps the scroll position, with a button to jump to new ones. animations respect reduced-motion preferences.

settings are stored in the user's application config directory (`cryptotape/cryptotape.sqlite`), with imported sounds alongside it. the proxy settings tab stores named profiles with host, port and optional credentials. the protocol is inferred: local unauthenticated proxies use SOCKS5, authenticated upstream proxies use HTTP CONNECT. direct routing is selected by default; a new profile is activated only when its checked-by-default "use this proxy" checkbox is left checked. saving an inactive profile does not change the selected route. selected proxies apply to both Binance REST and websocket traffic; terminal requests stay direct. there is no automatic direct-route fallback. a shadowsocks client can expose its local SOCKS5 port; raw shadowsocks encrypted endpoints are not supported by an ip/port alone.

## checks

```sh
go test ./internal/... .
go test -tags integration ./internal/binance -run TestPublicFuturesFeed -count=1
cd frontend && npm run lint && npm test && npm run build
```

the default suite uses local HTTP/websocket fixtures and temporary SQLite databases; it needs neither Binance access nor an installed terminal. it verifies:
- creating a rule through the settings form, edit/delete, save failures and retry;
- controller rule creation, immediate partial-window detection, persistence before event delivery, and restart recovery;
- running-engine configuration updates, turnover filters, cooldown and no rebound spam;
- REST startup, real websocket frame decoding, stale/future frame rejection and reconnect with a fresh baseline;
- terminal success/failure without losing notification/history, and all 750 requests during a market crash;
- overlay event delivery and the three-card limit.

`.github/workflows/check.yml` runs backend checks on windows and macos and frontend checks on every push/pull request. the macos job also runs the race detector. these jobs run once the workflow is pushed to GitHub; a windows cross-build on macos does not prove that windows tests passed.

native overlay placement/focus over fullscreen apps, actual audio output, real terminal behavior and end-to-end latency still need release checks on windows 11. automated component tests cannot certify those operating-system integrations.

the integration test contacts the live Binance futures API. latency and resource targets need profiling under full market load on the intended Windows 11 machine. Tiger uses a persistent local websocket at port 7819 by default and `SETLINKSYMBOL` with `BINANCE-FUT` / `FUTURES`. enable the local signal server in Tiger and link the target windows to the configured group (A by default); opening symbols does not require enabling trading. protocol: [Tiger API](https://support.tiger.com/razrabotka-dlya-tiger.trade-windows/tiger-api). terminal icons come from [Vataga](https://vataga.trading/_next/static/media/logo-white.1d49bd01.svg) and [Tiger](https://tiger.com/images/favicon.ico).

the five built-in sounds include the original `notification.mp3` for personal use; confirm distribution rights before public distribution.

the automated crash replay warms 750 markets, then feeds 82,500 volatile quotes with drops from 10% to 60% and sharp rebounds. it verifies 1,500 first-crossing alerts, no rebound spam, and complete SQLite cursor recovery. a separate local HTTP test verifies all 750 terminal commands. frontend tests cover 1/5/15/1,500 notifications, burst batching, and terminal-status recovery. these are component checks, not a Windows end-to-end latency measurement.

## interface languages

general settings supports english and russian (`settings.language`: `en` / `ru`). the selection persists in SQLite and updates both webviews without restarting the market feed. typed dictionaries live in `frontend/src/i18n/`; user alert names, tickers, terminal identifiers and custom sound filenames remain unchanged. external API diagnostic details retain their original content.
