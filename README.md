# PivotOnTheGO

PivotOnTheGO is a local-only pivoting and loot helper with a Ligolo-ng one-click installer (“Skiddie Mode”), a file server with per-file one-liners, route helpers, proxy profile storage, and a remote filesystem scout (SSH/SMB/Evil-WinRM; FTP stub). All UI/API traffic binds to `127.0.0.1`.

## Features
- **Ligolo-ng Skiddie Mode**: Downloads/installs proxy/agent to your app data dir and updates config.
- **File Server + Loot Browser**: Serves from a configurable directory (defaults to your loot dir) and generates per-file curl/PowerShell download commands.
- **Route Helper**: Builds `ip route add` commands.
- **SOCKS/Proxy Profiles**: Store local SOCKS/HTTP endpoints in browser localStorage.
- **Remote Filesystem Scout**: SSH/SMB/Evil-WinRM path enumeration (FILE|/path and DENIED| markers only), saved under loot.
- **Konami + Skiddie audio gags** (optional MP3s).

## Requirements
- Go 1.20+ (for build/run)
- Ligolo-ng dependencies (Skiddie Mode downloads the binaries)
- Optional: `ssh`, `smbclient`, `evil-winrm` in PATH for the FS Scout

## Build & Run
Using Make:
```bash
make build     # builds bin/pivotonthego
make install   # builds to ~/.local/bin/pivotonthego
make run       # go run ./cmd/ui
```
Without Make:
```bash
go build -o pivotonthego ./cmd/ui
./pivotonthego
```
Then open `http://127.0.0.1:8080/`.

## Paths & Data
- App data (preferred): `~/.local/share/PivotOnTheGO`
- Legacy fallback: `~/.local/share/SwissArmyToolkit` (used only if the new path is absent)
- Config: `~/.config/PivotOnTheGO/config.json` (fallback to legacy config path if it already exists)
- Loot dir (default file server root): `~/.local/share/PivotOnTheGO/loot`
- Ligolo binaries (Skiddie Mode): `~/.local/share/PivotOnTheGO/ligolo/`
- Audio (Skiddie/Konami): `~/.local/share/PivotOnTheGO/assets/media/.hidden/skiddiemode.mp3` and `konamisound.mp3`


## FS Scout notes
- SSH: uses `ssh find ...`; expects key/agent auth (password not injected).
- SMB: uses `smbclient //host/share -U user%pass -c "recurse; ls"`.
- Evil-WinRM: runs a PowerShell walker to the given depth.
- FTP: stub (not automated; will return not implemented).
- Results: `~/.local/share/PivotOnTheGO/loot/fs/<host>/<timestamp>_<protocol>_<mode>.txt`

## File Server / Loot Browser
- Defaults to the loot dir if config is empty.
- Per-file one-liners use the current Public IP + File Port inputs.
- Non-recursive listing.

## Route Helper & Proxy Profiles
- Route helper builds `sudo ip route add <subnet> dev <iface> [via <gw>]`.
- Proxy profiles are stored in browser localStorage (`swissarmykit_proxy_profiles`).

## Branding
- UI name: PivotOnTheGO
- Binary name: `pivotonthego`
- Go module/import paths remain unchanged for now.
