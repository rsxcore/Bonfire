# Nexus Mods page

Everything needed for the [Nexus Mods](https://www.nexusmods.com) page, field by field.

## 1. Mod details

| Field | Value |
|---|---|
| **Game** | Elden Ring (optionally also Dark Souls III and Sekiro, linking to the main page) |
| **Name** | `Bonfire - Save Manager` |
| **Category** | Utilities |
| **Version** | `2.0.0` |
| **Author** | rsxcore |
| **Language** | English |
| **Summary** | see below |
| **Description** | paste [`description.bbcode`](description.bbcode) |
| **Tags** | Utilities, Quality of Life, Save Game, User Interface, Tools |

**Summary** (the short text on the mod card, max 350 characters):

```
Quick save & quick load for FromSoftware games in one key. Named backups, every load undoable, SHA-256 verified restores. Supports Elden Ring, Nightreign, DS1/DS2/DS3, Sekiro, AC6 and Bloodborne (shadPS4). One small open-source .exe with nothing to install.
```

## 2. Images

Upload in this order from [`images/`](images); the first one becomes the thumbnail.

| File | Caption |
|---|---|
| `0-cover.png` | Bonfire - save manager for FromSoftware games |
| `1-main.png` | Your backups: quick save, named and automatic |
| `2-new-backup.png` | Name a backup after where you are |
| `3-restore.png` | Every restore is verified and can be undone |
| `4-keys.png` | All keys at a glance |

`0-cover.png` is rendered from [`cover.html`](cover.html).

## 3. Files

| Field | Value |
|---|---|
| **File** | `Bonfire-2.0.0.zip` (from the GitHub release) |
| **Name** | `Bonfire` |
| **Version** | `2.0.0` |
| **Category** | Main files |
| **Description** | `Unzip and run Bonfire.exe. Works best in Windows Terminal.` |
| **Mod manager download** | Off: it's a standalone tool, not a game mod |

## 4. Permissions

- Source: open source, MIT license, <https://github.com/rsxcore/Bonfire>
- Allow others to upload to other sites: **No** (link to this page or GitHub instead)
- Allow modifications and asset use: **Yes, with credit** (MIT)

## 5. Automatic updates

After the first upload, every `v*` tag uploads the new version to Nexus by itself (see `.github/workflows/release.yml`). One-time setup:

1. Nexus → avatar → **Site preferences → API Keys** → copy your personal API key.
2. On the mod's **Files** tab, turn on *Advanced* and copy the **file ID** of `Bonfire`.
3. In GitHub → repository → **Settings → Secrets and variables → Actions**:
   - secret `NEXUS_API_KEY`: the API key
   - variable `NEXUS_FILE_ID`: the file ID
   - variable `NEXUS_MOD_ID`: the mod ID from the page URL (for the changelog)

Until these are set, releases skip the Nexus upload and only publish on GitHub.
