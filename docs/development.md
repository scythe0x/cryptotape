# development

technical notes for contributors. installation and everyday use live in the [readme](../README.md).

## prerequisites

- go 1.26.10 or newer; `go.mod` pins the toolchain baseline.
- node.js 22 or newer with npm.
- wails cli matching the application dependency: `v3.0.0-beta.33`.
- windows 11 x64 and webview2 runtime for the published executable.
- macos with xcode command line tools for local development.

install the cli and ensure the go binaries directory is on `PATH` (`go env GOPATH`, then `bin/`):

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.33
```

## local development and builds

build the frontend before running go: the application embeds `frontend/dist`.

```sh
npm --prefix frontend ci
npm --prefix frontend run build
go run .
```

for frontend-only iteration, `npm --prefix frontend run dev` starts vite. native calls still require a wails runtime; a browser tab alone is not a working desktop application.

on windows, the standard release build includes the icon, manifest and version resources:

```powershell
$env:CGO_ENABLED = "0"
wails3 task build:windows
.\build\bin\Mythra.exe
```

`build:windows` installs frontend dependencies and builds assets. `build:windows:binary` reuses existing assets, generates the windows `.syso` resources and builds `build/bin/Mythra.exe` with ordinary go, the `production` tag, trimmed paths and the windows gui subsystem. no obfuscator or private build metadata is involved. keep version resources in sync before building a release.

on macos:

```sh
wails3 task run:mac
wails3 task build:mac
```

`run:mac` expects frontend assets to exist; `build:mac` builds them first and writes `build/bin/Mythra`. transparent notification windows require the `private_mac_apis` tag. the default macos build leaves the webview background opaque. macos is a local development target, outside the mac app store; the release workflow publishes windows only.

for a windows cross-build from macos, build the frontend, then run:

```sh
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 wails3 task build:windows:binary
```

a successful cross-build does not exercise windows-native behavior.

## checks and tests

run the standard checks from the repository root:

```sh
wails3 task check
```

or run each part separately:

```sh
go test ./internal/... ./cmd/... .
go vet ./internal/... ./cmd/... .
npm --prefix frontend run lint
npm --prefix frontend run typecheck
npm --prefix frontend test
npm --prefix frontend run build
```

the frontend build includes typechecking. ordinary tests use local http/websocket fixtures and temporary sqlite databases; they need neither binance access nor an installed terminal. coverage includes:

- rule creation, editing, deletion, save errors and retry through the settings form;
- startup without activation, including ignored legacy `license.dat` files;
- threshold crossings in partial fresh history, turnover filters, cooldown and no rebound spam;
- persistence before event delivery, history recovery and shutdown draining;
- websocket decoding, stale/future data rejection and reconnect with a fresh baseline;
- terminal success/failure, bounded queues and cancellation without losing stored alerts;
- feed/overlay delivery, the three-card notification limit and independent sound/banner preferences;
- clock geometry, timezone persistence and candle-boundary behavior.

run the race detector on a supported native host:

```sh
go test -race ./internal/...
```

opt-in checks that contact external services:

```sh
go test -tags integration ./internal/binance -run TestPublicFuturesFeed -count=1
go run ./cmd/listings-check -duration 30s
```

the listing probe reads the private local source config or `NLF_KEY`. it prints admission, plan/delay and arrival samples without creating alerts. never commit that key or include it in screenshots or logs.

native overlay placement/focus over fullscreen apps, actual sound, real terminal behavior, dpi, suspend/resume and end-to-end latency require acceptance checks on windows 11. component tests and cross-compilation do not verify those integrations.

## performance and diagnostics

```sh
bash scripts/perf.sh
# or
wails3 task perf
```

the script prints measurements in the terminal without result files. it requires go and bash, uses temporary sqlite databases and a local terminal fixture, and does not contact binance or operate native windows. a longer run:

```sh
bash scripts/perf.sh -benchtime=1s -count=5
```

| measurement | what it covers |
| --- | --- |
| `ProcessMarkets` | detector throughput for 750 markets and 2/10 rules sharing 1m/5m windows, below alert thresholds |
| `CombinedBookDecode` | time and allocations per json frame; compares the former two-pass decoder |
| `PersistAlertBatch` | committed batches of 1/64 alerts with production sqlite settings |
| `AlertPipelineBurst` | 750 price drops producing 1,500 persisted alerts and event-sink deliveries; excludes baseline preparation, browser rendering, sound and terminals |
| terminal burst test | 750 requests with 100ms fixture responses and the production 10-second freshness deadline |

lower time/allocation measurements are better; higher throughput is better. `B/op` measures allocated bytes, not retained memory. compare repeated runs on the same machine, go version and power settings. ordinary checks have no machine-dependent speed thresholds.

the crash replay warms 750 markets, feeds 82,500 volatile quotes and verifies 1,500 first-crossing alerts, no rebound spam and complete sqlite cursor recovery. these are component measurements, not physical screen-latency measurements.

diagnostics distinguish websocket round-trip ping from quote age. the main readout is the median of the last five successful pings, reset on disconnect; quote age stays in the tooltip. diagnostics include per-shard current age and a 30-second p99 histogram upper bound, rest snapshot age/errors, persistence/terminal backlogs and the latest 128 alert stage traces. frontend acknowledgements identify feed/overlay browser frames, not os presentation time.

additional audit models are opt-in:

```sh
CRYPTOTAPE_DYNAMIC_AUDIT=1 go test ./internal/storage -run '^TestDynamicLoadAudit$' -count=1 -v
go test ./internal/storage -run '^$' -bench BenchmarkDynamicPeakObservation -benchmem
cd frontend
VITE_DYNAMIC_AUDIT=1 npm test -- src/dynamic-audit.test.tsx
```

see the [market-data audit](market-data-audit.md) for measurements and limitations. dynamic percentages are test-only and are not enabled in the application. the [vps capacity study](vps-capacity.md) describes a historical experiment, not a deployed server-side detector.

## signal and data pipeline

- the go market engine runs independently of the wails ui. it subscribes to `@bookTicker` for trading usdt contracts above the lowest per-rule turnover threshold. rest refreshes turnover and contract membership every five minutes; failures retain the last good snapshot. clock, turnover and contract refreshes recover independently. new contracts join existing shards without restarting other symbols.
- price rules evaluate the bid/ask midpoint on every change, using 100ms extrema buckets indexed by raw binance event time. local monotonic time controls cooldown and freshness deadlines; clock correction affects quote age only. duplicate and out-of-order updates are rejected.
- the lookback is a maximum window, not a warmup requirement. the first crossing alerts immediately within available fresh history. cooldown equals the rule window, independently per ticker and rule, shared by both directions. crossings during cooldown are discarded and never replayed on expiry.
- a bounded queue accepts up to 8,192 candidates without waiting for sqlite. a writer commits batches of up to 64 before publishing notifications or scheduling actions. storage failures/overflow remain visible. shutdown drains accepted work with a 15-second persistence deadline and explicit errors for unsaved alerts.
- only one native sound plays at a time; the next alert replaces it. automatic terminal commands use separate bounded queues: 32 vataga workers and one tiger worker. request timeouts start after dequeue, but retain the original 10-second freshness deadline. distinct alerts are not coalesced; ambiguous failures are not retried.
- each price shard retries after 1, 2, 4 and 8 seconds, then waits for manual reconnect without stopping healthy shards. subscriptions require acknowledgement within five seconds; control messages are paced at five per second. rotation at 23h50m confirms a replacement before switching. a minute of connected, fresh data resets the retry budget.
- stale data and connection gaps reset the affected price window. movement across a gap is never inferred. the ui recovers missed alerts by sqlite cursor. burst rendering is batched without discarding history; reading older alerts preserves scroll position. animations respect reduced-motion preferences.

external listing feeds run independently of price detection. see [external listings](external-listings.md) for source authentication, event mapping, filtering, deduplication, exchange priority and direct-source feasibility.

## desktop, storage and integrations

all windows are frameless. use `WindowTitleBar` for regular windows and preserve frameless alert overlays. do not add native title bars or dialog windows. closing settings hides that window; closing the main window quits the application. closing the clock keeps the application running and persists its visibility separately.

keep the existing `cryptotape` data directory for compatibility. sqlite holds rules, settings and alert history; imported sounds live alongside it. windows uses `%APPDATA%\cryptotape`. replacing or relocating the executable for the same user preserves data. there is no installer or in-app updater.

proxy profiles store host, port and optional credentials locally. unauthenticated local proxies use socks5; authenticated upstream proxies use http connect. direct routing is the default. saving an inactive profile does not change the selected route. the selected proxy applies to binance rest and websocket traffic; terminal requests stay direct. failures do not silently fall back to a direct connection. shadowsocks requires a local socks5 client; a raw encrypted endpoint is not a socks5 proxy.

vataga uses a local http api at port `5005`; the binance futures route is `POST http://localhost:5005/api/open/zenica/binance/F/{SYMBOL}`. tiger uses a persistent local websocket, port `7819` and link group `A` by default. `SETLINKSYMBOL` selects `BINANCE-FUT`, `BYBIT`, `OKX` or `Gate.io` / `FUTURES`. enable tiger's local signal server and link the target windows to the configured group; opening symbols does not require enabling trading. protocol reference: [tiger api](https://support.tiger.com/razrabotka-dlya-tiger.trade-windows/tiger-api).

each rule has a color, sound, volume, turnover floor and independent terminal choices. rules remain active until deleted. banner duration is hidden, 5/15/30 seconds, 1/5 minutes or manual dismissal; new rules default to 15 seconds. hiding banners does not disable feed entries, sound or terminal actions. global sound and banner preferences persist independently.

typed english/russian dictionaries live in `frontend/src/i18n/`; changing `settings.language` updates the views without restarting feeds. user rule names, tickers and custom filenames are not translated. external diagnostic details retain their original content.

readme images show real react components with schema-validated demonstration data, not a live market session. update them when the visible interface changes. do not include real api keys, proxy credentials, account details or local filesystem paths in captures.

terminal icon sources: [vataga](https://vataga.trading/_next/static/media/logo-white.1d49bd01.svg) and [tiger](https://tiger.com/images/favicon.ico). exchange asset references live in [sources.md](../frontend/src/assets/exchanges/sources.md). the five built-in sounds include the original `notification.mp3` for personal use; confirm distribution rights before public distribution.

## ci and releases

`.github/workflows/check.yml` runs frontend/backend checks and a production windows build on pushes to `main` and pull requests. superseded checks are cancelled. manual workflow runs also run the macos race detector. feature-branch pushes without a pull request do not trigger automatic checks.

a version tag starts `.github/workflows/release.yml`: validate matching versions, check frontend/backend, build the tagged revision on windows, then create a draft github release with `Mythra.exe`. review the executable and publish that draft. there is no vps upload or download-link deployment. the repository's visibility controls access to release assets; keep application registration separate from github access permissions.

for each release, choose a new unused version and update all version fields. for example, after `v0.1.14`:

```sh
npm --prefix frontend version 0.1.15 --no-git-tag-version
```

set `fixed.file_version` and `info.0000.ProductVersion` in `build/windows/info.json` to `0.1.15`, run checks, and commit before tagging:

```sh
git push origin main
git tag -a v0.1.15 -m "mythra v0.1.15"
git push origin v0.1.15
```

the workflow rejects mismatched tags/application versions. a failed build does not publish a release. documentation-only commits can use `[skip ci]` when skipping automatic checks is intentional; do not create a version tag for documentation changes.
