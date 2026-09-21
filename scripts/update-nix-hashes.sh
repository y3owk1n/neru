#!/usr/bin/env bash
# Brings the hashes of the Nix package up to date: latestVersion in flake.nix
# and the four release zip hashes in nix/package.nix follow the newest GitHub
# release, and vendorHash follows go.mod and go.sum. Needs nix, gh and openssl.
# The nix-hashes workflow runs this, so nobody needs Nix on their own machine.
set -euo pipefail
cd "$(dirname "$0")/.."

repo="y3owk1n/neru"
pkg="nix/package.nix"

latest="$(gh release view --repo "$repo" --json tagName --jq .tagName)"
latest="${latest#v}"
current="$(sed -n 's/.*latestVersion = "\(.*\)";/\1/p' flake.nix)"

if [ "$latest" != "$current" ]; then
	echo "release $current -> $latest"
	for asset in darwin-arm64 darwin-amd64 linux-arm64 linux-amd64; do
		url="https://github.com/$repo/releases/download/v$latest/neru-$asset.zip"
		hash="sha256-$(curl -fsSL "$url" | openssl dgst -sha256 -binary | base64)"
		perl -0pi -e "s|(neru-$asset\.zip\";\n\s*sha256 = \")[^\"]*|\${1}$hash|" "$pkg"
	done
	perl -pi -e "s|latestVersion = \".*\";|latestVersion = \"$latest\";|" flake.nix
fi

# Nix prints the hash it got when the one it was given is wrong, which is the
# only way to learn a vendorHash. goModules is the vendor directory alone, so
# this does not compile neru.
old="$(sed -n 's/.*vendorHash = "\(.*\)";/\1/p' "$pkg")"
fake="sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
perl -pi -e "s|vendorHash = \".*\";|vendorHash = \"$fake\";|" "$pkg"
out="$(nix build .#source.goModules --no-link 2>&1 || true)"
new="$(printf '%s\n' "$out" | sed -n 's/.*got: *\(sha256-[A-Za-z0-9+\/=]*\).*/\1/p' | head -1)"
if [ -z "$new" ]; then
	perl -pi -e "s|vendorHash = \".*\";|vendorHash = \"$old\";|" "$pkg"
	printf '%s\n' "$out" >&2
	echo "nix build printed no vendor hash, see its output above" >&2
	exit 1
fi
perl -pi -e "s|vendorHash = \".*\";|vendorHash = \"$new\";|" "$pkg"
[ "$old" = "$new" ] || echo "vendorHash $old -> $new"
