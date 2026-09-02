# Knockout Backup

Knockout Networks edition of Kopia for MSP-managed Windows workstations and servers.

A technician (or the first-run wizard) provides only:

1. **Site ID** — becomes the storage subdirectory, for example `sites/acme-hq/`
2. **Passcode** — repository password, and optional shared enrollment PIN
3. **Profile** — one of the built-in Windows backup choices

The S3 or B2 destination is preconfigured in `knockout-destination.json`. Each client gets an isolated prefix in the same bucket. Nightly snapshots start automatically while Knockout Backup (or `kopia server start --ui`) is running.

## Destination file

Copy `knockout/destination.example.json` to one of:

- `KNOCKOUT_DESTINATION_FILE`
- `%APPDATA%\kopia\knockout-destination.json`
- next to the installer / `kopia.exe`
- `app/resources/knockout-destination.json` before building the Windows installer

### Amazon S3

```json
{
  "type": "s3",
  "bucket": "kn-client-backups",
  "endpoint": "s3.us-east-1.amazonaws.com",
  "region": "us-east-1",
  "accessKeyID": "...",
  "secretAccessKey": "...",
  "prefixBase": "sites/"
}
```

### Backblaze B2 (recommended: S3-compatible API)

```json
{
  "type": "s3",
  "bucket": "kn-client-backups",
  "endpoint": "s3.us-west-004.backblazeb2.com",
  "region": "us-west-004",
  "accessKeyID": "<B2 key ID>",
  "secretAccessKey": "<B2 application key>",
  "prefixBase": "sites/"
}
```

Native B2 (`"type": "b2"` with `keyID` / `key`) is also supported.

Environment variables override file values: `KNOCKOUT_STORAGE_TYPE`, `KNOCKOUT_BUCKET`, `KNOCKOUT_ENDPOINT`, `KNOCKOUT_REGION`, `KNOCKOUT_ACCESS_KEY_ID`, `KNOCKOUT_SECRET_ACCESS_KEY`, `KNOCKOUT_B2_KEY_ID`, `KNOCKOUT_B2_KEY`, `KNOCKOUT_PREFIX_BASE`.

### Optional shared enrollment PIN

Set `enrollmentPasscodeSHA256` to `SHA256(passcode)` if every install should use the same technician PIN. The passcode is still the repository password.

## Backup profiles

| ID | Typical use | Schedule | VSS | What is included |
| --- | --- | --- | --- | --- |
| `workstation` | Office PCs | 2:00 AM | when available | Documents, Desktop, Pictures, Videos, Music, Downloads, Favorites, OneDrive |
| `workstation-full` | Laptops that need the whole profile | 2:00 AM | when available | Entire user profile minus caches |
| `server` | File / app servers | 1:30 AM | always | Data volumes, `C:\Shares`, `C:\inetpub` |
| `server-system` | Servers that also need C: | 1:30 AM | always | `C:\` plus data volumes, excluding Windows junk |

Only paths that exist on the machine are registered. Add more with `--path`.

Retention defaults:

- Workstations: 14 daily / 8 weekly / 12 monthly / 3 annual
- Servers: 30 daily / 12 weekly / 24 monthly / 7 annual

## CLI

```
kopia knockout profiles
kopia knockout setup --site-id acme-hq --passcode '...' --profile workstation
kopia knockout status
```

Silent Windows enroll (RMM):

```
powershell -File installer\windows\enroll.ps1 -SiteId acme-hq -Passcode '...' -Profile workstation
```

## Windows installer

1. Put real bucket credentials in `app/resources/knockout-destination.json`
2. Build the engine and Electron shell:

```
make kopia-ui
```

On Windows this produces `dist/kopia-ui/KnockoutBackup-Setup-<version>.exe`.

First launch opens the Knockout enrollment card. After site ID + passcode, the tray app stays running and takes the first missed/nightly snapshot.

## What we still need from you

- Production S3 or B2 bucket name, region/endpoint, and an access key that can read/write the whole bucket (or a prefix)
- Whether you want a shared technician PIN (`enrollmentPasscodeSHA256`) or a unique passcode per site
- Preferred default profile if most endpoints are workstations (current default: `workstation`)
- Code-signing certificate for the Windows installer (optional but recommended)
- Brand artwork if you want the tray/installer icons replaced beyond the current KN mark
