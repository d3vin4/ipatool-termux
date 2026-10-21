# ipatool for Android / Termux

Port của [ipatool](https://github.com/majd/ipatool) (tác giả: majd, fork [DrWhax/ipatool](https://github.com/DrWhax/ipatool)) chạy được trên **Android / Termux**.

Bản này dựa trên nhánh `feature/fairplay-upstream-framework` của upstream, nên có thêm lệnh **`fairplay-decrypt`** — giải mã IPA đã personalize **offline hoàn toàn**, không cần máy jailbreak, không cần credential, không cần network.

> Bản gốc của tác giả: [`README.upstream.md`](README.upstream.md). Phần khác biệt cho Android: [`TERMUX.md`](TERMUX.md). Hướng dẫn dùng đầy đủ: [`TERMUX-GUIDE.md`](TERMUX-GUIDE.md).

## Cài đặt

```shell
pkg install git
git clone https://github.com/d3vin4/ipatool-termux.git
cd ipatool-termux
CGO_ENABLED=1 go build -o ipatool .
install -m 0755 ipatool "$PREFIX/bin/ipatool"
```

`CGO_ENABLED=1` là **bắt buộc**: `purego` nạp thư viện Unicorn runtime qua `cgo` trên Android.

Kiểm tra:

```shell
ipatool --version
```

## Dùng nhanh

```shell
# 1. đăng nhập (hỏi Apple ID, mật khẩu, 2FA, passphrase keychain)
ipatool auth login

# 2. tìm app
ipatool search "zalo"

# 3. tải + giải mã trong một lệnh
ipatool download -i 579523206 -o Zalo.ipa --fairplay-decrypt

# hoặc tách 2 bước
ipatool download -i 579523206 -o Zalo.ipa
ipatool fairplay-decrypt --input Zalo.ipa
```

Kết quả: `Zalo.decrypted.ipa` + `Zalo.decrypted.json` (report private, mode `0600`).

IPA đã giải mã là **IPA phân tích** — phải re-sign trước khi cài.

## MCP server

`ipatool mcp` chạy MCP server qua stdio, expose 6 tool: `search_apps`, `download_app`, `list_app_versions`, `get_version_metadata`, `list_purchases`, `purchase_app`.

### opencode

Thêm vào `opencode.json`:

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

`claude_desktop_config.json`:

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

Cần `ipatool auth login` chạy trước trong terminal — MCP server dùng account đã lưu, và sẽ yêu cầu bạn chạy `ipatool auth login` nếu chưa đăng nhập.

Kiểm tra server sống:

```shell
printf '%s\n' \
 '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"probe","version":"1.0"}}}' \
 '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
 '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | ipatool mcp
```

## Khác biệt so với bản gốc (vì Android)

| Vấn đề | Cách xử lý trong port này |
| --- | --- |
| Unicorn là bản manylinux → bionic không load được | Patch ELF tại chỗ (soname, symbol version, 4 alias glibc) rồi mới `dlopen` |
| Android giấu MAC address → mọi lệnh fail | Sinh 1 địa chỉ locally-administered, lưu ở `~/.ipatool/machine-id` |
| Upstream chặn FairPlay trên Android | Mở gate (private state nằm trong sandbox nên còn an toàn hơn) |
| Android cấm syscall `link()` | Publish output bằng `renameat2(RENAME_NOREPLACE)` thay hardlink |

Chi tiết: [`TERMUX.md`](TERMUX.md).

## Giới hạn đã biết

**Model SINF là theo từng đời FairPlay.** Upstream nhúng sẵn một model pack
(`macos26-26A428-current-sinf`) — đúng với đời FairPlay mà tác giả dựng model.
IPA được tạo ở đời khác sẽ không giải mã được, và tool **fail closed** thay vì
xuất ra rác:

```
SINF does not match the embedded current FairPlay profile: plaintext atom 0 ("key ") is unreadable: ...
[model macos26-26A428-current-sinf sha256 c9b7c488..., selector aea196be:00000005, priv payload 448 bytes]
```

Nếu gặp lỗi này, dùng model đúng đời:

```shell
ipatool fairplay-decrypt --input App.ipa --model /path/to/model.bin
```

Model pack có định dạng `FPS26A4`: magic 8 byte, version, length, `fairplayd` hash 32 byte, AES key 16 byte, whitening 16 byte, rồi 16 bảng permutation 256 byte. Tool tự kiểm tra cấu trúc; model nhúng vẫn được kiểm tra SHA-256 để chống sửa đổi.

## Build từ source

```shell
git clone https://github.com/d3vin4/ipatool-termux.git
cd ipatool-termux
CGO_ENABLED=1 go build -ldflags="-X github.com/majd/ipatool/v2/cmd.version=$(date +%Y.%m.%d)-termux" -o ipatool .
install -m 0755 ipatool "$PREFIX/bin/ipatool"
```

Test:

```shell
go generate ./...
go test ./...
```

## License

Port này giữ license MIT của upstream. Xem [`LICENSE`](LICENSE).

Lưu ý: thư viện Unicorn (GPL) **không** được bundle vào binary — nó vẫn được
tải về lúc chạy như upstream, nên binary vẫn thuần MIT.
