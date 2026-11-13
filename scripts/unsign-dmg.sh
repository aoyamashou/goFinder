#!/bin/sh
# DMG 不签名，内部应用仍保留 Apple_self_dev 签名。
# 用法：scripts/unsign-dmg.sh [--arch arm64|amd64|all] [--upload]
# 默认只处理 arm64；所有目标校验完成后才上传 draft release。
set -eu
cd "$(dirname "$0")/.."
usage() {
	echo '用法：scripts/unsign-dmg.sh [--arch arm64|amd64|all] [--upload]' >&2
	exit 1
}
arch=arm64
upload=false
while [ "$#" -gt 0 ]; do
	case "$1" in
		--arch) [ "$#" -ge 2 ] || usage; arch=$2; shift 2 ;;
		--upload) upload=true; shift ;;
		*) usage ;;
	esac
done
case "$arch" in
	arm64|amd64) arches=$arch ;;
	all) arches='arm64 amd64' ;;
	*) usage ;;
esac
ver=$(sed -n 's/.*"version": *"\([^"]*\)".*/\1/p' mygo.json)
[ -n "$ver" ] || { echo '无法读取版本号' >&2; exit 1; }
for target in $arches; do
	dmg="build/darwin-$target/gofinder-$ver-$target.dmg"
	[ -f "$dmg" ] || { echo "找不到 $dmg" >&2; exit 1; }
done
for tool in hdiutil codesign shasum; do
	command -v "$tool" >/dev/null || { echo "找不到工具 $tool" >&2; exit 1; }
done
if [ "$upload" = true ]; then
	# 部分上传失败仍必须留在 draft，禁止改动已公开版本。
	[ "$(gh release view "v$ver" --repo aoyamashou/goFinder --json isDraft --jq .isDraft)" = true ] || {
		echo "v$ver 不是 draft release，拒绝替换附件" >&2; exit 1;
	}
fi
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
if [ -f build/SHA256SUMS ]; then
	cp build/SHA256SUMS "$tmp/sums"
else
	: > "$tmp/sums"
fi
# 先暂存并校验所有转换结果，再替换本地文件；任何目标失败都不会上传。
for target in $arches; do
	name="gofinder-$ver-$target.dmg"
	dmg="build/darwin-$target/$name"
	hdiutil convert "$dmg" -format ULMO -o "$tmp/$name" >/dev/null
	if codesign -dv "$tmp/$name" 2>/dev/null; then
		echo "DMG 仍带签名：$name" >&2; exit 1
	fi
	hdiutil verify "$tmp/$name" >/dev/null
	shasum -a 256 "$tmp/$name" > "$tmp/digest"
	read -r sum rest < "$tmp/digest"
	awk -v name="$name" '$2 != name' "$tmp/sums" > "$tmp/next-sums"
	printf '%s  %s\n' "$sum" "$name" >> "$tmp/next-sums"
	mv "$tmp/next-sums" "$tmp/sums"
done
for target in $arches; do
	name="gofinder-$ver-$target.dmg"
	mv "$tmp/$name" "build/darwin-$target/$name"
	echo "已生成不签名的 $name"
done
sort -k2 "$tmp/sums" > build/SHA256SUMS
if [ "$upload" = true ]; then
	for target in $arches; do
		gh release upload "v$ver" "build/darwin-$target/gofinder-$ver-$target.dmg" --repo aoyamashou/goFinder --clobber
	done
	gh release upload "v$ver" build/SHA256SUMS --repo aoyamashou/goFinder --clobber
	echo "已替换 draft v$ver 的 DMG 和 SHA256SUMS"
fi
