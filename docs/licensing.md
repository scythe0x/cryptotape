# licensing and basic executable protection

windows releases use go 1.26.10 and garble v0.17.0. internal application packages have identifier and literal obfuscation; the wails rpc surface remains stable. private obfuscation seeds are written to `build/private/` and retained separately from the public executable. v0.1.11 restores this profile at the owner's request after comparing antivirus results with ordinary go builds. neither profile guarantees zero detections. licensing and cryptographic checks remain unchanged. the deeper rea/ghidra audit was stopped in favor of possible server-side computation.

## access contract

- an online signed response is required on every windows launch and every hour thereafter. five attempts have an 8-second request timeout and delays of 2, 4, 8 and 16 seconds, bounded by a 70-second cycle.
- ed25519 signatures bind each result to a fresh nonce, the equipment digest, server time, expiry and license id. https verification remains enabled. changing hosts/dns or replaying a response does not produce a valid grant.
- expiry uses server-relative lifetime and windows uptime, including suspend; local wall-clock changes do not extend access. the periodic supervisor blocks expired sessions even while renewal requests are retrying.
- one telegram account receives one permanent random key. its initial 30 days start on issue. repeated requests never create a new key or reset expiry.
- the first activation binds smbios uuid and an available usable board serial. deleting app files, reinstalling windows or replacing a disk does not release that binding. another board cannot use the bound key. unavailable hardware identity fails closed. spoofed firmware identifiers remain a limitation.
- key storage uses windows dpapi. the frontend receives state and errors, never the saved key. before licensing succeeds, useful views stay unmounted and native desktop methods reject calls.
- revocation, expiry or exhausted retries hides useful windows, interrupts feeds/audio/terminal requests, drops pending actions and returns to activation. previously transmitted terminal commands cannot be recalled.
- macos is a development target without this windows licensing flow; it is not published by the release job.

## server

`https://scalping.mythra.pw` is served by caddy. the api listens only at `127.0.0.1:9087`; `/healthz` checks database availability. the go service runs the api and telegram long polling under a dedicated `mythra-licensing` user. bot commands and the commands menu are configured automatically. there is no public administrative api.

configuration is `/etc/mythra-licensing.env`, root-readable only. it contains the bot token, signing seed, database encryption key, hmac lookup key and database path. none belong in git, build metadata, client binaries or logs. the public verification key is intentionally committed in the client. preserve the server configuration securely together with database backups: an encrypted database alone cannot recover issued keys.

```sh
systemctl status mythra-licensing caddy
journalctl -u mythra-licensing --since '1 hour ago'
systemctl start mythra-licensing-backup.service
```

daily consistent sqlite backups use `vacuum into`, are stored under `/var/lib/mythra-licensing/backups/`, and are retained for seven days. verify a restored copy with `pragma integrity_check` before using it. these are local backups, not protection against loss of the entire vps.

run administrative operations as root, loading the existing protected configuration without printing it:

```sh
set -a
. /etc/mythra-licensing.env
set +a
/opt/mythra-licensing/license-admin -action extend -telegram-id TELEGRAM_USER_ID -days 30
/opt/mythra-licensing/license-admin -action revoke -telegram-id TELEGRAM_USER_ID
```

extension preserves the key and hardware binding and sets `expires_at = max(expires_at, now) + days`; it also clears revocation. payments and tariff selection are not implemented.

## infrastructure change

the existing vpn container's external tcp port moved from 443 to 10443; its internal port remains 443. vpn client profiles using the former 443 endpoint must use 10443. the existing 8443 mapping and local admin mapping are unchanged. caddy owns public ports 80/443 for the project subdomain only; the apex domain is not configured as a project host.

the prior vpn compose file is retained at `/opt/mythra-licensing/rollback/vpn-compose-before.yaml`. restoring that mapping requires stopping caddy first because both would otherwise need port 443; it also makes the license api unavailable, so active clients eventually fail closed.

## verification boundaries

protocol, expiry, replay, hardware ownership, concurrent activation, key issuance, renewal, backup restoration and native/frontend gating have automated tests. local windows cross-compilation does not exercise real smbios firmware, dpapi persistence, window decorations, dpi or suspend/resume on a windows machine. those require a windows acceptance run before treating the executable as a fully validated public release.

## telegram administration

admin id `5766296513` is fixed in `internal/licenseserver/admin.go`. commands require both that sender id and the matching private chat; forwarded text or group messages do not grant rights. telegram's chat-scoped menu shows these commands to the admin only.

- `/add <id> <days>` adds 1–3650 days to `max(expiry, now)`.
- `/set <id> <days>` sets expiry to `now + days`; zero expires the subscription immediately, without setting a revocation or blocking flag.
- `/get <id>` returns the license id, telegram id, key, creation/expiry times, remaining days, revocation/binding flags and all stored hardware hashes. raw motherboard serials are not stored. large exports are delivered as a json document.
- `/reset_hwid <id>` removes every hardware alias and clears the binding flag, preserving the key, expiry and revocation state.
- `/public_add <days>` adds days to every existing key, including keys whose subscription was cancelled with `/set 0`. these subscriptions regain access normally. legacy server-side revocations remain independent of expiry.

ids are telegram user ids. commands for an unknown user return an error rather than creating an implicit free trial. valid day ranges are 1–3650 for additions and 0–3650 for setting a term.

notification texts are:

- additions: `Вам начислено N дополнительных дней доступа в Mythra`
- positive set: `Время истечения подписки Mythra изменено на N дней`
- set zero or administrative revocation: `Ваша подписка Mythra была деактивирована`

mutations and encrypted outgoing messages are committed together. processed telegram update ids prevent duplicate additions on retries or restarts. the persistent notification worker respects telegram retry delays; a recipient who blocked the bot does not stop other deliveries. sent/failed status is retained in `bot_outbox`; failures are logged without keys. telegram delivery cannot be made exactly-once across an ambiguous network response, but replay never adds subscription days twice. cli extension/revocation also queues the corresponding notification.

the migration adds `admin_updates` and `bot_outbox` (schema version 2) without changing existing license keys or hardware bindings. desktop clients receive changes on their next license check; restarting the app checks immediately.

## public executable download

caddy serves `/download`, `/download/` and `/Mythra.exe` from `/srv/mythra/releases/Mythra.exe`, with an attachment filename and caching disabled. license api routes keep using the local reverse proxy.

the release workflow first creates a draft for checking the exact executable. after publishing that draft, the download workflow downloads its `Mythra.exe` asset and uploads it over ssh as `mythra-release`. repository secrets `MYTHRA_RELEASE_SSH_KEY` and `MYTHRA_RELEASE_KNOWN_HOSTS` hold the deployment credential and pinned host key. the ssh key uses a forced command, `deploy/licensing/publish-release.py` installed as `/usr/local/bin/mythra-publish-release`; it cannot open a shell or forward ports. uploads are limited to 100 mib, checked for dos/pe signatures and atomically replace the public file. the workflow downloads the public url and compares its bytes with the github release asset.
