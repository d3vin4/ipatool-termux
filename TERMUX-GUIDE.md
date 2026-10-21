# ipatool trên Termux — hướng dẫn A→Z

Bản đang cài: `ipatool version 2.0.0-termux` tại `$PREFIX/bin/ipatool`.

## 0. Yêu cầu trước

| Thứ | Ghi chú |
| --- | --- |
| Internet | tải Unicorn runtime (22 MB) + asset `storeagent` từ Apple, chỉ lần đầu |
| Apple ID | bắt buộc, hỗ trợ 2FA |
| Terminal thật | các prompt mật khẩu dùng raw mode, pipe vào sẽ lỗi `inappropriate ioctl` |

Kiểm tra nhanh:

```shell
ipatool --version
```

## 1. Đăng nhập

```shell
ipatool auth login
```

Các prompt theo thứ tự:

1. `enter Apple ID (email address or phone number):` → email Apple
2. `enter password:` → hiện `*` khi gõ (mật khẩu Apple ID)
3. `enter 2FA code:` → chỉ hiện khi tài khoản bắt buộc 2FA, lấy mã 6 số từ thiết bị trusted
4. `enter passphrase to unlock ~/.ipatool/account (this is separate from your Apple ID password):`
   → **passphrase riêng** để mã hóa file credential. Không phải mật khẩu Apple. Tự chọn và nhớ lại.

Lần đầu sẽ chậm vài phút (tải + patch Unicorn, tải asset FairPlay). Các lần sau dùng cache.

### Không interactive (script/CI)

```shell
ipatool auth login \
  -e "you@icloud.com" \
  -p "MAT_KHAU_APPLE" \
  --auth-code 123456 \
  --keychain-passphrase "PASSPHRASE_KEYCHAIN"
```

> `-p` sẽ nằm trong shell history. Chỉ dùng trong script tự động; bình thường để tool hỏi.

### Kiểm tra / đăng xuất

```shell
ipatool auth info      # xem account hiện tại
ipatool auth revoke    # xóa credential
```

Credential lưu ở `~/.ipatool/account` (mode `0600`, mã hóa JWE PBES2-HS256 + A256GCM bằng passphrase).

## 2. Tìm app

```shell
ipatool search "instagram"
ipatool search "instagram" -l 10                 # nhiều kết quả hơn
ipatool search "instagram" --platform ipad
ipatool search "instagram" --format json | jq '.apps[].bundleId'
```

`search` **cần đã đăng nhập** (tool đọc account trước). Lấy `trackId` (đây là app ID) / `bundleId` / `trackName` từ kết quả.

> Ở output JSON, field tên là `trackId` / `trackName` (không phải `appId` / `name`).

## 3. Xem version (tuỳ chọn)

```shell
ipatool list-versions -i 284882215
ipatool list-versions -b com.bundle.id
ipatool get-version-metadata -i 284882215 --external-version-id <id>
```

## 4. Tải app

```shell
# cơ bản
ipatool download -i 284882215 -o Instagram.ipa

# -o là thư mục → tool tự đặt tên <bundleID>_<appID>_<version>.ipa
ipatool download -i 284882215 -o ~/downloads/

# dùng bundle identifier (app bị gỡ khỏi store vẫn tải được nếu còn ID)
ipatool download -b com.bundle.id

# app có phí: tự mua license nếu chưa có
ipatool download -i 284882215 --purchase

# bản cũ
ipatool download -i 284882215 --external-version-id <id>

# nền tảng khác
ipatool download -i 284882215 --platform appletv
ipatool download -i 284882215 --platform watchos
ipatool download -i 284882215 --platform visionos
ipatool download -i 284882215 --platform macos
```

Mặc định platform = iPhone + iPad.

### App có phí

Nếu chưa có license, lệnh báo `license is required`. Xử lý:

```shell
ipatool purchase -i 284882215            # mua license
ipatool list-purchases                  # xem đã mua gì
ipatool download -i 284882215 --purchase # tự mua luôn
```

## 5. Decrypt

### macOS — có decrypt thật, tự động

```shell
ipatool download -i <macAppID> --platform macos -o App.pkg
```

Tool tải `.pkg` đã mã hoá → emulate `storeagent` của Apple bằng Unicorn để giải mã → validate checksum từng entry XAR → xuất `.pkg` **đã giải mã**.

Lưu ý: nếu app đó thực chất là app iOS chạy được trên Mac (Apple Silicon), tool tự nhận ra và dùng luồng iOS, **không** decrypt.

Kiểm tra: `xar -tf App.pkg | head` (trên máy có xar), hoặc `pkgutil --check-signature App.pkg` (trên macOS).

### iOS / iPadOS / tvOS / watchOS / visionOS — CÓ decrypt local (nhánh fairplay)

Nhánh `feature/fairplay-upstream-framework` bổ sung pipeline FairPlay thuần Go, **offline hoàn toàn** — không cần máy jailbreak, không cần credential, không cần network:

```shell
# giải mã một IPA đã personalize
ipatool fairplay-decrypt --input App.ipa
# -> App.decrypted.ipa  +  App.decrypted.json (report private, mode 0600)

# hoặc tải xong giải mã luôn
ipatool download -i 284882215 -o App.ipa --fairplay-decrypt
```

Flag: `--input` (bắt buộc), `--output`, `--report`, `--force`.

Cơ chế: dùng model SINF hiện tại nhúng sẵn trong binary, không mở phiên giải mã nào, không gọi hardware. Nó **fail closed** nếu SINF của IPA không khớp model.

Ràng buộc quan trọng:

- Input phải là **personalized IPA** — tức IPA tải bằng chính account của bạn (dùng `ipatool download` với account đó), nếu không SINF sẽ không khớp.
- **"The decrypted analysis IPA must be recursively re-signed before installation."** — output là IPA để phân tích, cần re-sign trước khi cài.
- Report `.json` là private state (mode 0600), chứa thông tin nhạy cảm về key — đừng upload.

Trên Android, tool bảo vệ thêm: `checkPlatform()` chỉ cho phép macOS/Linux ở upstream. Bản port này mở gate cho Android (xem `TERMUX.md` §3).

### Các tool khác (nếu bạn không dùng nhánh này)

| Tool | CLI | Ghi chú |
| --- | --- | --- |
| [EthanArbuckle/fairplay-decrypt](https://github.com/EthanArbuckle/fairplay-decrypt) | `python decrypt.py <in.ipa> <out.ipa> --credential credential.bin` | Cần credential 16 byte lấy từ iPhone jailbreak. Không hỗ trợ license iOS 26+ |
| [londek/ipadecrypt](https://github.com/londek/ipadecrypt) | `ipadecrypt decrypt <bundle-id\|path-to-local-ipa>` | Điều khiển iPhone jailbreak qua SSH. Xử lý được iOS 26 |

## 6. Xử lý sự cố

| Triệu chứng | Nguyên nhân / cách sửa |
| --- | --- |
| `could not be found in the keyring` | chưa login, hoặc sai passphrase keychain |
| `keychain passphrase is required when not running in interactive mode` | thiếu `--keychain-passphrase` |
| `license is required` | app có phí → `--purchase` hoặc `ipatool purchase` |
| `2FA code is required` | chạy lại với `--auth-code` |
| `password token expired` | tool tự login lại bằng password đã lưu; nếu fail thì `ipatool auth login` lại |
| Download lỗi bất thường | `ipatool <lệnh> --verbose` |
| Keychain hỏng / sai passphrase | `rm ~/.ipatool/account` rồi `ipatool auth login` (**giữ `machine-id`**) |
| `inappropriate ioctl for device` khi hỏi mật khẩu | đừng pipe stdin; chạy trực tiếp trong terminal |

## 7. Script hoá

```shell
ipatool --format json --non-interactive auth login \
  -e "$APPLE_ID" -p "$APPLE_PASS" \
  --auth-code "$CODE" --keychain-passphrase "$KPASS"

ipatool --format json --non-interactive search "app name" \
  --keychain-passphrase "$KPASS" | jq -r '.apps[] | "\(.trackId) \(.bundleId)"'

ipatool --non-interactive download -i "$APP_ID" -o out.ipa \
  --keychain-passphrase "$KPASS"
```

Nhớ: `--non-interactive` yêu cầu `--keychain-passphrase` cho mọi lệnh đọc credential.

## 8. Vòng đời tổng quát

```
ipatool auth login
      │  (Apple ID + 2FA + passphrase keychain)
      ▼
~/.ipatool/account  ──┐
                      │
ipatool search        │  đọc account
ipatool download      │  đọc account
      │               │
      ▼               ▼
  KBSync (Unicorn)  ticket từ ent/download
      │               │
      └───────┬───────┘
              ▼
      Apple CDN  ──►  .ipa  ──► fairplay-decrypt ──► .decrypted.ipa
                    .pkg  (đã decrypt sẵn, chỉ macOS)
```
