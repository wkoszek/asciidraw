#!/usr/bin/env bash
# Sign and notarize the macOS binary. GoReleaser runs this on the universal
# binary before it goes into the archive; see .goreleaser.yaml.
#
#   scripts/macos-sign.sh <path-to-binary>
#
# MACOS_SIGN=0 skips it (make snapshot does). Anything else signs, and fails
# the release if signing or notarizing fails.
#
# Needs, on this Mac:
#   - the "Developer ID Application" certificate in the login keychain
#   - a notarytool profile (default sense-notary; it is per Apple account, not
#     per app). Make one with:
#       xcrun notarytool store-credentials sense-notary --apple-id adam@koszek.com \
#         --team-id QQ5A9Q7C7Z --password "$(opc read op://infra/apple-notary-wkoszek/token)"
set -euo pipefail

BIN=${1:?usage: macos-sign.sh <binary>}
IDENTITY=${MACOS_IDENTITY:-"Developer ID Application: Adam Koszek (QQ5A9Q7C7Z)"}
PROFILE=${NOTARY_PROFILE:-sense-notary}
BUNDLE_ID=com.koszek.asciidraw

say() { printf '  macos-sign: %s\n' "$*" >&2; }
die() { say "✗ $*"; exit 1; }

if [ "${MACOS_SIGN:-1}" = 0 ]; then
	say "MACOS_SIGN=0, leaving $BIN unsigned"
	exit 0
fi

[ "$(uname -s)" = Darwin ] || die "signing needs macOS (set MACOS_SIGN=0 to skip)"
security find-identity -v -p codesigning | grep -qF "$IDENTITY" ||
	die "signing identity not in keychain: $IDENTITY"
xcrun notarytool history --keychain-profile "$PROFILE" >/dev/null 2>&1 ||
	die "no notarytool profile '$PROFILE'; see the top of $0"

# Notarization needs the hardened runtime and a secure timestamp.
codesign --force --sign "$IDENTITY" --identifier "$BUNDLE_ID" \
	--options runtime --timestamp "$BIN"
codesign --verify --strict "$BIN" || die "signature does not verify"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
/usr/bin/ditto -c -k --keepParent "$BIN" "$work/notarize.zip"

say "notarizing $(basename "$BIN"); this takes a minute or two"
out=$(xcrun notarytool submit "$work/notarize.zip" --keychain-profile "$PROFILE" \
	--wait --output-format json) || die "notarytool failed: $out"
status=$(printf '%s' "$out" | /usr/bin/python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')
if [ "$status" != Accepted ]; then
	id=$(printf '%s' "$out" | /usr/bin/python3 -c 'import json,sys; print(json.load(sys.stdin).get("id",""))')
	xcrun notarytool log "$id" --keychain-profile "$PROFILE" >&2 || true
	die "notarization status: $status"
fi

# A bare Mach-O can't be stapled; Gatekeeper checks the ticket online.
spctl -a -vvv -t install "$BIN" 2>&1 | grep -q "Notarized Developer ID" ||
	die "Gatekeeper does not see the notarization"
say "signed and notarized"
