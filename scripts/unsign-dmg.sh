#!/bin/sh
# 把 mygo build 生成的 DMG 重新转换为不带签名的 DMG（mygo 没有跳过 DMG 签名的选项）。
# 自签证书签过的 DMG 会被 Gatekeeper 直接拒绝打开；不签名的 DMG 可以正常挂载，
# 里面的 goFinder.app 仍然用 Apple_self_dev 签名。同时更新 build/SHA256SUMS。
# 用法：scripts/unsign-dmg.sh [--upload]   --upload 时替换 GitHub draft release 上的 DMG 和 SHA256SUMS。
set -eu
cd "$(dirname "$0")/.."
ver=$(sed -n 's/.*"version": *"\([^"]*\)".*/\1/p' mygo.json)
name="gofinder-$ver-arm64.dmg"
dmg="build/darwin-arm64/$name"
[ -f "$dmg" ] || { echo "找不到 $dmg" >&2; exit 1; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
hdiutil convert "$dmg" -format ULMO -o "$tmp/$name" >/dev/null
mv "$tmp/$name" "$dmg"
if codesign -dv "$dmg" 2>/dev/null; then echo "DMG 仍带签名" >&2; exit 1; fi
hdiutil verify "$dmg" >/dev/null
sum=$(shasum -a 256 "$dmg" | cut -d' ' -f1)
if [ -f build/SHA256SUMS ]; then
	grep -v "  $name\$" build/SHA256SUMS > "$tmp/sums" || true
	echo "$sum  $name" >> "$tmp/sums"
	sort -k2 "$tmp/sums" > build/SHA256SUMS
fi
echo "已生成不签名的 $name ($sum)"
if [ "${1:-}" = "--upload" ]; then
	gh release upload "v$ver" "$dmg" build/SHA256SUMS --clobber
	echo "已替换 v$ver 的 DMG 和 SHA256SUMS"
fi
