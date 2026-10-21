# ipatool on Termux — A to Z

Installed build: `ipatool version 2.0.0-termux-fairplay` in `$PREFIX/bin/ipatool`.

This is the Android port of the upstream `feature/fairplay-upstream-framework`
branch, so it also provides the `fairplay-decrypt` command and the portable
FairPlay pipeline.

## 0. Requirements

| Item | Note |
| --- | --- |
| Internet | downloads the Unicorn runtime (22 MB) and the Apple assets on first use |
| Apple ID | required, 2FA is supported |
| A real terminal | the password prompts use raw mode, piping stdin fails with `inappropriate ioctl` |

Check the install:

```shell
ipatool --version
```

## 1. Sign in

```shell
ipatool auth login
```

Four prompts, in order:

1. `enter Apple ID (email address or phone number):` → your Apple ID
2. `enter password:` → shows `*` while you type (your Apple ID password)
3. `enter 2FA code:` → only when the account requires 2FA, take the six digit
   code from your trusted device
4. `enter passphrase to unlock ~/.ipatool/account (this is separate from your Apple ID password):`
   → a **separate passphrase** that encrypts the credential file. Pick one and
   remember it.

The first run takes a few minutes: it downloads and patches the Unicorn
runtime and fetches the Apple FairPlay assets. Later runs reuse the cache.

### Non-interactive (scripts, CI)

```shell
ipatool auth login \
  -e "you@icloud.com" \
  -p "APPLE_PASSWORD" \
  --auth-code 123456 \
  --keychain-passphrase "KEYCHAIN_PASSPHRASE"
```

> `-p` ends up in the shell history. Only use it in automation; otherwise let
> the tool prompt.

### Check and sign out

```shell
ipatool auth info      # show the stored account
ipatool auth revoke    # remove the credentials
```

The credential lives in `~/.ipatool/account` (mode `0600`, encrypted with the
passphrase).

## 2. Find an app

```shell
ipatool search "instagram" -l 10
```

`search` needs a signed in account, because it reads the account first. The
result carries `trackId` (the app ID), `bundleId` and `trackName`. For JSON:

```shell
ipatool --format json search "instagram" | jq -r '.apps[] | "\(.trackId) \(.bundleId)"'
```

## 3. List versions (optional)

```shell
ipatool list-versions -i 284882215
ipatool list-versions -b com.bundle.id
ipatool get-version-metadata -i 284882215 --external-version-id <id>
```

## 4. Download

```shell
# basic
ipatool download -i 284882215 -o Instagram.ipa

# -o may be a directory, the tool then picks <bundleID>_<appID>_<version>.ipa
ipatool download -i 284882215 -o ~/downloads/

# by bundle identifier (works for delisted apps that still have an ID)
ipatool download -b com.bundle.id

# paid app: acquire the licence first
ipatool download -i 284882215 --purchase

# an older build
ipatool download -i 284882215 --external-version-id <id>

# other platforms
ipatool download -i 284882215 --platform appletv
ipatool download -i 284882215 --platform watchos
ipatool download -i 284882215 --platform visionos
ipatool download -i 284882215 --platform macos
```

The default platform is iPhone and iPad.

### Paid apps

Without a licence the command reports `license is required`. Then either:

```shell
ipatool purchase -i 284882215            # buy the licence
ipatool list-purchases                  # what the account owns
ipatool download -i 284882215 --purchase # buy and download in one go
```

## 5. Decrypt

### macOS — decrypted automatically

```shell
ipatool download -i <macAppID> --platform macos -o App.pkg
```

The tool downloads the encrypted `.pkg`, decrypts it with the emulated Apple
`storeagent`, validates the checksum of every XAR entry and publishes the
decrypted package.

Note: when the app is really an iOS app that runs on Apple silicon, the tool
recognises it and uses the iOS path, so the package is **not** decrypted.

### iOS / iPadOS / tvOS / watchOS / visionOS — decrypted locally, offline

The feature branch adds a pure Go FairPlay pipeline. It needs no jailbroken
device, no credential and no network:

```shell
# decrypt an already personalized IPA
ipatool fairplay-decrypt --input App.ipa
# -> App.decrypted.ipa  +  App.decrypted.json (private report, mode 0600)

# or download and decrypt in one command
ipatool download -i 284882215 -o App.ipa --fairplay-decrypt
```

Flags: `--input` (required), `--output`, `--report`, `--force`, `--model`.

How it works: the embedded current-SINF model is used to derive the logical
key, no decryption session is opened and no hardware call is made. It **fails
closed** when the SINF does not match the model.

Important constraints:

* The input has to be a **personalized IPA**, meaning one that was downloaded
  with the very same account. A SINF from another account does not match.
* **"The decrypted analysis IPA must be recursively re-signed before
  installation."** The output is an analysis IPA and has to be re-signed.
* The `.json` report is private state (mode `0600`) and contains key material.
  Do not upload it.

On Android, upstream gates the pipeline to macOS and Linux. This port opens the
gate, see [`TERMUX.md`](TERMUX.md) section 3.

### Verify the result

The report is the source of truth:

```shell
jq '{complete, warnings, pages: .decrypted_page_count,
     cryptid: [.images[] | {m: .member, before: .cryptid_before, after: .cryptid_after}],
     plausible: [.images[] | .first_page_structurally_plausible]}' App.decrypted.json
```

What you want to see:

* `"complete": true`
* `"cryptid_after": 0` (it was 1 before)
* `"warnings": []` or empty
* `first_page_structurally_plausible: true`, which means the page decrypts to
  plausible ARM64 code instead of noise

`plaintext_verified` and `signature_verified` are **always false**: the tool
does not verify a signature, so the IPA has to be re-signed before it can be
installed.

### When the model does not match

```
SINF does not match the embedded current FairPlay profile: plaintext atom 0 ("key ") is unreadable: ...
[model macos26-26A428-current-sinf sha256 c9b7c488..., selector aea196be:00000005,
 SINF {user 4, key 4, iviv 16, righ 88, name 256, priv 448}, priv payload 448 bytes]
```

The embedded model covers one FairPlay generation while the tool ships
macOS 27 assets, so a current SINF does not decode. Supply a matching model:

```shell
ipatool fairplay-decrypt --input App.ipa --model /path/to/model.bin
```

### Other tools (when not using this branch)

| Tool | CLI | Note |
| --- | --- | --- |
| [EthanArbuckle/fairplay-decrypt](https://github.com/EthanArbuckle/fairplay-decrypt) | `python decrypt.py <in.ipa> <out.ipa> --credential credential.bin` | Needs a 16 byte credential taken from a jailbroken iPhone. Does not support the iOS 26+ license format |
| [londek/ipadecrypt](https://github.com/londek/ipadecrypt) | `ipadecrypt decrypt <bundle-id\|path-to-local-ipa>` | Drives a jailbroken iPhone over SSH. Handles iOS 26 |

## 6. Troubleshooting

| Symptom | Fix |
| --- | --- |
| `could not be found in the keyring` | not signed in, or the keychain passphrase is wrong |
| `keychain passphrase is required when not running in interactive mode` | pass `--keychain-passphrase` |
| `license is required` | add `--purchase` |
| `2FA code is required` | run the command again with `--auth-code` |
| `password token expired` | the tool signs in again with the stored password; run `ipatool auth login` if that fails |
| `SINF does not match the embedded current FairPlay profile` | the IPA is from another FairPlay generation, see the model section |
| `FairPlay decryption is supported only on macOS...` | an old binary is installed, reinstall the package |
| `inappropriate ioctl for device` at a password prompt | do not pipe stdin, run it in a terminal |
| Odd download failures | add `--verbose` |

## 7. Scripting

```shell
ipatool --format json --non-interactive auth login \
  -e "$APPLE_ID" -p "$APPLE_PASS" \
  --auth-code "$CODE" --keychain-passphrase "$KPASS"

ipatool --format json --non-interactive search "app name" \
  --keychain-passphrase "$KPASS" | jq -r '.apps[] | "\(.trackId) \(.bundleId)"'

ipatool --non-interactive download -i "$APP_ID" -o out.ipa \
  --fairplay-decrypt --keychain-passphrase "$KPASS"
```

`--non-interactive` requires `--keychain-passphrase` for every command that
reads the credential.

## 8. The whole flow

```
ipatool auth login
      │  (Apple ID + 2FA + keychain passphrase)
      ▼
~/.ipatool/account  ──┐
                      │
ipatool search        │  reads the account
ipatool download      │  reads the account
      │               │
      ▼               ▼
  KBSync (Unicorn)  ticket from ent/download
      │               │
      └───────┬───────┘
              ▼
      Apple CDN  ──►  .ipa  ──► fairplay-decrypt ──► .decrypted.ipa
                    .pkg  (decrypted already, macOS only)
```
