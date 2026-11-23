#!/bin/sh
# Conventional commits decide the next version. No outside tools: git, awk, curl.
#
#   scripts/release.sh lint FILE      check one commit message (the commit-msg hook)
#   scripts/release.sh lint A..B      check every commit in a range (CI)
#   scripts/release.sh log            the commits since the last tag, and what each one bumps
#   scripts/release.sh next           print the next version, e.g. v0.2.0
#   scripts/release.sh draft          put a draft section for it at the top of CHANGELOG.md
#   scripts/release.sh tag            tag HEAD with it, push the tag, wait for GitHub
#
# How the version moves (https://semver.org):
#   fix:, perf:                 patch   v1.2.3 -> v1.2.4
#   feat:                       minor   v1.2.3 -> v1.3.0
#   feat!:, BREAKING CHANGE:    major   v1.2.3 -> v2.0.0
# Before v1.0.0 a breaking change bumps the minor version, not the major.
# Other types (docs, test, refactor, build, ci, chore, style, revert) don't
# release anything on their own. BUMP=patch|minor|major overrides the result.
set -eu

TYPES='feat fix perf refactor docs test build ci chore style revert'
TYPES_RE=$(echo "$TYPES" | tr ' ' '|')
HEADER_RE="^($TYPES_RE)(\([^()]+\))?!?: [^ ]"
MAX_HEADER=72

die() { echo "✗ $*" >&2; exit 1; }

# The subject of a message on stdin, minus comments and anything below the
# scissors line that git commit -v adds.
subject() {
	awk '/^# -+ >8 -+/ { exit } /^#/ { next } NF { print; exit }'
}

# lint_header HEADER: say what's wrong with it, if anything.
lint_header() {
	h=$1
	case $h in
	"Merge "* | 'Revert "'* | "fixup! "* | "squash! "* | "amend! "*) return 0 ;;
	esac
	if ! printf '%s\n' "$h" | grep -Eq "$HEADER_RE"; then
		cat >&2 <<EOF
✗ Commit subjects look like "type: what changed" or "type(scope): what changed".
    got:   $h
    types: $TYPES
    feat bumps the minor version; fix and perf bump the patch.
    For a breaking change, write "feat!:" or add a "BREAKING CHANGE:" line.
    Example: feat(clipboard): paste images as text shapes
EOF
		return 1
	fi
	if [ ${#h} -gt $MAX_HEADER ]; then
		echo "✗ Subject is ${#h} characters; keep it to $MAX_HEADER: $h" >&2
		return 1
	fi
}

cmd_lint() {
	[ $# -eq 1 ] || die "usage: release.sh lint FILE | A..B"
	case $1 in
	*..*)
		bad=0
		for c in $(git rev-list --no-merges "$1"); do
			lint_header "$(git log -1 --format=%s "$c")" || { echo "    in $(git log -1 --format=%h "$c")" >&2; bad=1; }
		done
		return $bad
		;;
	*) lint_header "$(subject <"$1")" ;;
	esac
}

last_tag() {
	git describe --tags --abbrev=0 --match 'v[0-9]*.[0-9]*.[0-9]*' 2>/dev/null || true
}

range() {
	t=$(last_tag)
	if [ -n "$t" ]; then echo "$t..HEAD"; else echo HEAD; fi
}

# bump_of SHA: major, minor, patch, none, or unknown (not a conventional commit).
bump_of() {
	msg=$(git log -1 --format=%B "$1")
	h=$(printf '%s\n' "$msg" | head -n 1)
	if ! printf '%s\n' "$h" | grep -Eq "$HEADER_RE"; then
		echo unknown
	elif printf '%s\n' "$h" | grep -Eq '^[a-z]+(\([^()]+\))?!:' ||
		printf '%s\n' "$msg" | grep -Eq '^BREAKING[ -]CHANGE:'; then
		echo major
	else
		case $h in
		feat*) echo minor ;;
		fix* | perf*) echo patch ;;
		*) echo none ;;
		esac
	fi
}

cmd_log() {
	t=$(last_tag)
	echo "Since ${t:-the first commit}:"
	for c in $(git rev-list --no-merges --reverse "$(range)"); do
		printf '  %-8s %s\n' "$(bump_of "$c")" "$(git log -1 --format='%h %s' "$c")"
	done
	v=$(cmd_next 2>&1) || true
	echo "Next: $v"
}

cmd_next() {
	level=0 unknown=0
	for c in $(git rev-list --no-merges "$(range)"); do
		case $(bump_of "$c") in
		major) level=3 ;;
		minor) [ $level -ge 2 ] || level=2 ;;
		patch) [ $level -ge 1 ] || level=1 ;;
		unknown) unknown=$((unknown + 1)) ;;
		esac
	done
	case ${BUMP:-} in
	major) level=3 ;;
	minor) level=2 ;;
	patch) level=1 ;;
	"")
		[ $unknown -eq 0 ] || die "$unknown commit(s) since the last tag aren't conventional, so the version is a guess. Say which with BUMP=patch|minor|major."
		;;
	*) die "BUMP must be patch, minor or major, not $BUMP" ;;
	esac
	[ $level -gt 0 ] || die "Nothing to release: no feat, fix or perf commits since $(last_tag)."

	t=$(last_tag)
	set -- $(echo "${t:-v0.0.0}" | sed 's/^v//; s/[-+].*//' | tr . ' ')
	major=$1 minor=$2 patch=$3
	# Before 1.0, a breaking change is a minor bump.
	[ "$major" -gt 0 ] || [ $level -lt 3 ] || level=2
	case $level in
	3) echo "v$((major + 1)).0.0" ;;
	2) echo "v$major.$((minor + 1)).0" ;;
	1) echo "v$major.$minor.$((patch + 1))" ;;
	esac
}

# Strip "type(scope)!: " and capitalize what's left.
describe() {
	d=$(printf '%s\n' "$1" | sed -E "s/^($TYPES_RE)(\([^()]+\))?!?: //")
	printf '%s%s\n' "$(printf %s "$d" | cut -c1 | tr '[:lower:]' '[:upper:]')" "$(printf %s "$d" | cut -c2-)"
}

cmd_draft() {
	v=$(cmd_next)
	if grep -q "^## $v " CHANGELOG.md; then
		echo "CHANGELOG.md already has a section for $v."
		return 0
	fi
	tmp=$(mktemp)
	trap 'rm -f "$tmp" "$tmp".*' EXIT
	for c in $(git rev-list --no-merges --reverse "$(range)"); do
		s=$(git log -1 --format=%s "$c")
		case $(bump_of "$c") in
		major) describe "$s" >>"$tmp.major" ;;
		minor) describe "$s" >>"$tmp.minor" ;;
		patch) describe "$s" >>"$tmp.patch" ;;
		unknown) echo "$s" >>"$tmp.unknown" ;;
		esac
	done
	{
		echo "## $v ($(date +%Y-%m-%d))"
		for part in "major:Breaking changes" "minor:New" "patch:Fixed" "unknown:Other"; do
			f="$tmp.${part%%:*}"
			[ -s "$f" ] || continue
			echo
			echo "### ${part#*:}"
			echo
			sed 's/^/- /' "$f"
		done
		echo
	} >"$tmp"
	awk -v f="$tmp" '
		!done && /^## / { while ((getline l < f) > 0) print l; done = 1 }
		{ print }
		END { if (!done) { print ""; while ((getline l < f) > 0) print l } }
	' CHANGELOG.md >"$tmp.out"
	mv "$tmp.out" CHANGELOG.md
	echo "Drafted $v at the top of CHANGELOG.md."
}

# Tag HEAD, push the tag to tig, and wait for the mirror to carry it to GitHub.
# Running it again after a failure picks up where it stopped.
cmd_tag() {
	t=$(git describe --tags --exact-match --match 'v[0-9]*' HEAD 2>/dev/null || true)
	if [ -z "$t" ]; then
		[ -z "$(git status --porcelain)" ] || die "The tree has uncommitted changes. Commit or stash them first."
		[ "$(git rev-parse --abbrev-ref HEAD)" = main ] || die "Release from main."
		git fetch -q tig main
		[ "$(git rev-parse HEAD)" = "$(git rev-parse tig/main)" ] ||
			die "HEAD isn't tig's main. Push first: git push tig main"
		t=$(cmd_next)
		grep -q "^## $t " CHANGELOG.md ||
			die "CHANGELOG.md has no section for $t. Run make changelog, edit it, and commit."
		git tag -a "$t" -m "asciidraw $t"
		echo "Tagged $t."
	fi
	git push -q tig "$t"

	: "${GITEA_TOKEN_REF:?}" "${GITHUB_TOKEN_REF:?}"
	gitea=$(opc read -n "$GITEA_TOKEN_REF")
	github=$(opc read -n "$GITHUB_TOKEN_REF")
	curl -fsS -o /dev/null -X POST -H "Authorization: token $gitea" \
		https://tig.koszek.com/api/v1/repos/wkoszek/asciidraw/push_mirrors-sync ||
		die "Couldn't ask tig to sync its GitHub mirror."
	want=$(git rev-parse "$t")
	i=0
	while [ $i -lt 40 ]; do
		got=$(curl -fsS -H "Authorization: token $github" \
			"https://api.github.com/repos/wkoszek/asciidraw/git/refs/tags/$t" 2>/dev/null |
			awk -F'"' '/"sha"/ { print $4; exit }') || true
		if [ "$got" = "$want" ]; then
			echo "$t is on GitHub."
			return 0
		fi
		i=$((i + 1))
		sleep 3
	done
	die "$t didn't reach GitHub in two minutes. Check the push mirror on tig."
}

[ $# -ge 1 ] || die "usage: release.sh lint|log|next|draft|tag"
c=$1
shift
case $c in
lint) cmd_lint "$@" ;;
log) cmd_log ;;
next) cmd_next ;;
draft) cmd_draft ;;
tag) cmd_tag ;;
*) die "unknown command: $c" ;;
esac
