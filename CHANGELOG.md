# Changelog

## [v0.2.5] - 2025-12-20

**Changed**
- PreUpdateVersion is now set in uPrep as well, also added more logging to help with debugging

## [v0.2.4] - 2025-12-20

**Added**
- Delete guild feature in admin settings with confirmation modal

## [v0.2.3] - 2025-12-20

**Fixed**
- VAAPI probe should now work on AMD, was too smoll b4

## [v0.2.2] - 2025-12-20

**Fixed**
- settings dashboard auth should now work on mobile

## [v0.2.1] - 2025-12-20

**Added**
- `service set` command to set config values for bootstrapping server

## [v0.2.0] - 2025-12-20

### 🎉 Stable Beta Release

First beta release for friends to use while development continues.

### Features

**Discord Bot**
- Anti link-rot: Automatically downloads and archives media from Reddit, YouTube Shorts, and RedGifs
  - Long YouTube video downloads require admin confirmation
- Auto-expand: Optionally replaces link-only messages with embedded media (per-user toggles)
- AI Chat: Ollama-powered conversational responses with intent classification
- Favorite message command: Right-click any message to save media to your favorites channel
- Download message command: Get direct download links for any archived media

**Settings Dashboard**
- Web-based admin panel with auth that bootstraps out-of-band via Discord
- Per-guild configuration (bot channel, favorites channel, backup settings, etc.)
- Per-channel AI chat toggles
- User permission management (admin, backup access, AI access)
- Dark/light theme with system preference detection

**Media Processing**
- Video compression via ffmpeg with hardware acceleration detection (VAAPI/NVENC)
- Image compression to AVIF format
- Configurable Ollama endpoint for AI features

### WIP

- Message/attachment backup system
- Reddit gallery posts
