# ipatool for Android / Termux

A port of [ipatool](https://github.com/majd/ipatool) (by majd, mirrored at [DrWhax/ipatool](https://github.com/DrWhax/ipatool)) that runs on **Android / Termux**.

It is based on the upstream `feature/fairplay-upstream-framework` branch, so it also ships the **`fairplay-decrypt`** command: a portable FairPlay pipeline that decrypts a personalized IPA **fully offline** — no jailbroken device, no credential, no network.

> Upstream documentation: [`README.upstream.md`](README.upstream.md). What the port changes: [`TERMUX.md`](TERMUX.md). Full walkthrough: [`TERMUX-GUIDE.md`](TERMUX-GUIDE.md).

## Install

### Package

```shell
dpkg -i ipatool-termux_2.0.0-termux-fairplay_aarch64.deb
```

Download it from the [releases page](https://github.com/d3vin4/ipatool-termux/releases). The package installs into the Termux prefix, so `ipatool` ends up in `$PREFIX/bin`.

> The Termux `dpkg` needs to install into the prefix. If `dpkg -i` reports `Read-only file system` while creating `./usr`, point it at the prefix:
>
> ```shell
> dpkg --root="$PREFIX" -i ipatool-termux_2.0.0-termux-fairplay_aarch64.deb
> ```

### From source

```shell
pkg install git
git clone https://github.com/d3vin4/ipatool-termux.git
cd ipatool-termux
CGO_ENABLED=1 go build -o ipatool .
install -m 0755 ipatool "$PREFIX/bin/ipatool"
```

`CGO_ENABLED=1` is **required**: `purego` loads the Unicorn runtime library through `cgo` on Android.

To build the package instead:

```shell
CGO_ENABLED=1 go build -ldflags="-X github.com/majd/ipatool/v2/cmd.version=2.0.0-termux-fairplay" -o ipatool .
./tools/build-deb.sh
```

Verify:

```shell
ipatool --version
```

## Quick start

```shell
# 1. sign in (prompts for Apple ID, password, 2FA code, keychain passphrase)
ipatool auth login

# 2. find an app
ipatool search "zalo"

# 3. download and decrypt in one command
ipatool download -i 579523206 -o Zalo.ipa --fairplay-decrypt

# or in two steps
ipatool download -i 579523206 -o Zalo.ipa
ipatool fairplay-decrypt --input Zalo.ipa
```

Outputs: `Zalo.decrypted.ipa` and `Zalo.decrypted.json` (a private report, mode `0600`).

The decrypted IPA is an **analysis IPA** — it has to be re-signed before installation.

## MCP server

`ipatool mcp` serves the App Store over MCP on stdio and exposes six tools: `search_apps`, `download_app`, `list_app_versions`, `get_version_metadata`, `list_purchases` and `purchase_app`.

### opencode

Add to `opencode.json`:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "ipatool": {
      "type": "local",
      "command": ["ipatool", "mcp"],
      "enabled": true
    }
  }
}
```

### Claude Desktop

In `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "ipatool": {
      "command": "ipatool",
      "args": ["mcp"]
    }
  }
}
```

Run `ipatool auth login` in a terminal first: the MCP server uses the stored account and asks you to run the login yourself when authentication is required. Purchases cover free apps only.

Probe the server:

```shell
printf '%s\n' \
 '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"probe","version":"1.0"}}}' \
 '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
 '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | ipatool mcp
```

## Differences from upstream (because of Android)

| Problem | How the port handles it |
| --- | --- |
| The published Unicorn build is manylinux, so bionic refuses to load it | The library is patched in place after the download (sonames, symbol versions, four glibc aliases) and then loaded |
| Android hides the interface hardware address, so every command failed | A locally administered address is generated once and stored in `~/.ipatool/machine-id` |
| Upstream gates the FairPlay pipeline to macOS and Linux | The gate is opened for Android: the private state lives in the application sandbox, where the permission bits are enforced even more strictly |
| Android denies the `link` syscall | The decrypted package is published with `renameat2(RENAME_NOREPLACE)` instead of a hard link |

Details: [`TERMUX.md`](TERMUX.md).

## Known limitations

**The SINF model is tied to a FairPlay generation.** Upstream embeds one model pack
(`macos26-26A428-current-sinf`) that matches the generation its author derived it
from, while the tool now ships macOS 27 (`26A434`) assets. A SINF from another
generation is not decoded, and the tool fails closed instead of producing
garbage:

```
SINF does not match the embedded current FairPlay profile: plaintext atom 0 ("key ") is unreadable: ...
[model macos26-26A428-current-sinf sha256 c9b7c488..., selector aea196be:00000005,
 SINF {user 4, key 4, iviv 16, righ 88, name 256, priv 448}, priv payload 448 bytes]
```

The reported geometry is what tells a mismatched generation apart from a corrupt
IPA: the model expects a `name` atom of 264 bytes, while a current SINF carries
256 and a `priv` payload of 448 without the eight byte zero trailer.

A matching model pack can be supplied without rebuilding:

```shell
ipatool fairplay-decrypt --input App.ipa --model /path/to/model.bin
```

A model pack has the `FPS26A4` layout: an eight byte magic, a version, the
length, a 32 byte `fairplayd` hash, a 16 byte AES key, a 16 byte whitening
value and sixteen 256 byte permutation tables. The tool validates the
structure; the embedded pack is additionally checked against its SHA-256 so it
stays tamper-evident.

## Build and test

```shell
go generate ./...
go test ./...
```

## License

This port keeps the upstream MIT license, see [`LICENSE`](LICENSE).

The Unicorn library (GPL) is **not** bundled into the binary. It is downloaded
at runtime exactly like upstream does, so the binary itself stays MIT.
