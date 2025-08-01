# Halsey

Wip docs.

Requires ffmpeg and yt-dlp to be installed and on PATH.

## Install

### Linux

Default (installs latest version to /usr/local/bin) (recommended):
```sh
curl -sSfL https://raw.githubusercontent.com/Data-Corruption/halsey/main/scripts/install.sh | bash
```

### Windows With WSL

Open a powershell terminal as administrator:
```powershell
Set-ExecutionPolicy Bypass -Scope Process -Force; iex "& { $(irm https://raw.githubusercontent.com/Data-Corruption/halsey/main/scripts/install.ps1) }"
```

## Usage

```sh
halsey -h
```

Set your bot token via `halsey config bot-token <token>`.
Configure anything else as needed.
Run it `halsey run`.