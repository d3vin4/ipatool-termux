# Running ipatool on Android / Termux

This tree is upstream `ipatool` (module `github.com/majd/ipatool/v2`, cloned from
https://github.com/DrWhax/ipatool.git) with an Android port layered on top.

## Build

```shell
CGO_ENABLED=1 go build -ldflags="-X github.com/majd/ipatool/v2/cmd.version=$(date +%Y.%m.%d)-termux" -o ipatool .
install -m 0755 ipatool "$PREFIX/bin/ipatool"
```

`CGO_ENABLED=1` is required: `purego` loads the Unicorn runtime library through
`cgo` on Android.

## What the port changes

### 1. Unicorn runtime library (the hard part)

`ipatool auth login` signs its requests with Apple's SAP signer, which emulates
the `storeagent` binary with the Unicorn CPU emulator. Upstream downloads a
prebuilt `libunicorn.so.2` and `dlopen`s it. The published artifacts are
manylinux builds, and bionic refuses them:

* `DT_NEEDED` names glibc sonames (`libc.so.6`, `libm.so.6`, `libpthread.so.0`)
* the library demands `GLIBC_2.17` symbol versions
* it imports four glibc internal aliases bionic does not have

`internal/sap/unicorn/patch_android.go` rewrites the library in place after it
has been downloaded and checksum-verified:

| patch | effect |
| --- | --- |
| `DT_NEEDED` sonames | `libc.so.6` → `libc.so`, `libm.so.6` → `libm.so`, `libpthread.so.0` → `libc.so` |
| `.gnu.version` | every versioned reference reset to `VER_NDX_GLOBAL` |
| `DT_VERNEED` / `DT_VERNEEDNUM` | zeroed, so the loader ignores the verneed table |
| symbol names | `__strdup` → `strdup`, `__getauxval` → `getauxval`, `__sigsetjmp` → `sigsetjmp`, `__errno_location` → `__errno` |

Every edit reuses an offset that already exists in the file, so the ELF layout
stays valid and the patch is safe to apply twice. The patched library is cached
separately under `~/.cache/ipatool/unicorn/<version>/<sha>/bionic/` and is
verified against a pinned digest before use.

The GPL Unicorn library is still downloaded at runtime rather than bundled, so
the binary itself stays under the upstream MIT license.

### 2. Device identity

Android does not expose interface hardware addresses to applications: the
kernel reports `02:00:00:00:00:00` for every interface and SELinux blocks
`/sys/class/net`. Upstream rejects that dummy address and fails with
`could not find network interfaces with a valid mac address`, which breaks
every command because the address is part of the device identity.

`pkg/util/machine/mac_address_android.go` keeps the upstream behaviour when a
usable address is exposed and otherwise falls back to a locally administered
address that is generated once and stored in the state directory
(`~/.ipatool/machine-id`), so the identity stays stable across runs.

### 3. Credentials

`byteness/keyring` selects the file backend on Android: the Secret Service
backend is not even registered because there is no D-Bus session bus. The
passphrase for the keychain file is prompted for interactively, or supplied
with `--keychain-passphrase` in non-interactive runs.

## Tests

```shell
go generate ./...
go test ./pkg/util/machine/... ./internal/sap/unicorn/...
```

Two extra tests are skipped unless their fixtures are provided:

```shell
IPATOOL_UNICORN_FIXTURE=/path/to/libunicorn.so.2 go test ./internal/sap/unicorn/ -run TestPatchBionicFixture
IPATOOL_UNICORN_EMULATION=1 go test ./internal/sap/unicorn/ -run TestEngineEmulation
```

The second one downloads the runtime library, patches it, loads it and executes
a small x86-64 program through the engine.
