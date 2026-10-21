#!/usr/bin/env bash
# Builds a Termux .deb for ipatool so it can be installed with dpkg -i.
#
# The package installs into $PREFIX, which is where Termux keeps its tree, so
# the payload paths are relative to the prefix and carry no leading usr/.
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="${1:-2.0.0-termux-fairplay}"
PACKAGE="ipatool-termux"
ARCH="$(dpkg --print-architecture 2>/dev/null || echo arm64)"
BUILD="$(mktemp -d)"
trap 'rm -rf "$BUILD"' EXIT

if [ ! -f ipatool ]; then
	echo "ipatool binary not found, run: CGO_ENABLED=1 go build -o ipatool ." >&2
	exit 1
fi

mkdir -p "$BUILD/DEBIAN" "$BUILD/bin" "$BUILD/share/doc/$PACKAGE"
chmod 0755 "$BUILD" "$BUILD/DEBIAN"

install -m 0755 ipatool "$BUILD/bin/ipatool"
install -m 0644 LICENSE "$BUILD/share/doc/$PACKAGE/LICENSE"
install -m 0644 README.md "$BUILD/share/doc/$PACKAGE/README.md"

INSTALLED_SIZE="$(du -ks "$BUILD/bin" "$BUILD/share" | awk '{s+=$1} END {print s}')"

cat >"$BUILD/DEBIAN/control" <<EOF
Package: $PACKAGE
Version: $VERSION
Architecture: $ARCH
Installed-Size: $INSTALLED_SIZE
Section: utils
Priority: optional
Maintainer: Bao <baoamai6@gmail.com>
Homepage: https://github.com/d3vin4/ipatool-termux
Description: ipatool for Android/Termux
 Search the App Store, download app packages and decrypt personalized IPAs
 with the portable FairPlay pipeline.
 .
 This package is a Termux port of ipatool (https://github.com/majd/ipatool).
 It adds the bionic compatible Unicorn runtime, a stable device identity and
 the FairPlay decryption pipeline.
EOF

cat >"$BUILD/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
echo "ipatool installed. Run 'ipatool auth login' to sign in to the App Store."
echo "The Unicorn runtime and the Apple FairPlay assets are downloaded on first use."
exit 0
EOF

chmod 0755 "$BUILD/DEBIAN/postinst"

OUTPUT="$PWD/${PACKAGE}_${VERSION}_${ARCH}.deb"
rm -f "$OUTPUT"
dpkg-deb --build --root-owner-group "$BUILD" "$OUTPUT" >/dev/null

echo "built $OUTPUT"
dpkg-deb --info "$OUTPUT" | sed -n '1,12p'
