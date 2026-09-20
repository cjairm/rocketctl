#!/bin/bash
# Ticket helper. Templates the boilerplate so callers pass only dynamic info.
#
#   ticket.sh new bug|feat "title" [-d detail] [-r file:line] [-p parent#]
#   ticket.sh find "title terms"        # title-only search
#   ticket.sh ls [bug|feat]
#   ticket.sh link child# parent#
#
# Board: https://github.com/users/cjairm/projects/3 (needs: gh auth refresh -s project)

set -euo pipefail

PROJECT_OWNER=cjairm
PROJECT_NUMBER=3

die() {
	echo "error: $*" >&2
	exit 1
}

# Compact one-line-per-issue output to keep reads cheap.
fmt_list='{{range .}}{{.number}}	{{.state}}	{{.title}}{{"\n"}}{{end}}'

cmd_new() {
	local kind=${1:-} title=${2:-} detail='' ref='' parent='' label
	shift 2 2>/dev/null || die 'usage: ticket.sh new bug|feat "title" [-d detail] [-r file:line] [-p parent#]'
	[ -n "$title" ] || die 'title required'

	case "$kind" in
	bug) label=bug ;;
	feat) label=enhancement ;;
	*) die 'kind must be bug or feat' ;;
	esac

	while [ $# -gt 0 ]; do
		case "$1" in
		-d)
			detail=${2:-}
			shift 2
			;;
		-r)
			ref=${2:-}
			shift 2
			;;
		-p)
			parent=${2:-}
			shift 2
			;;
		*) die "unknown flag: $1" ;;
		esac
	done

	local body=''
	if [ -n "$detail" ]; then body+="$detail"$'\n'; fi
	if [ -n "$ref" ]; then body+=$'\n'"Ref: \`$ref\`"$'\n'; fi
	if [ -n "$parent" ]; then body+=$'\n'"Parent: #${parent#\#}"$'\n'; fi
	if [ -z "$body" ]; then body="(no detail)"; fi

	local url
	url=$(gh issue create --title "$title" --label "$label" --body "$body")
	echo "$url"

	# Board add is best-effort: missing the project scope must not fail the ticket.
	gh project item-add "$PROJECT_NUMBER" --owner "$PROJECT_OWNER" --url "$url" >/dev/null 2>&1 ||
		echo "note: not added to board (run: gh auth refresh -s project)" >&2
}

cmd_find() {
	[ -n "${1:-}" ] || die 'usage: ticket.sh find "title terms"'
	gh issue list --state all --limit 20 --search "$1 in:title" --json number,state,title -t "$fmt_list"
}

cmd_ls() {
	local args=(--state open --limit 30 --json number,state,title -t "$fmt_list")
	case "${1:-}" in
	bug) args+=(--label bug) ;;
	feat) args+=(--label enhancement) ;;
	'') ;;
	*) die 'usage: ticket.sh ls [bug|feat]' ;;
	esac
	gh issue list "${args[@]}"
}

cmd_link() {
	local child=${1:-} parent=${2:-}
	[ -n "$child" ] && [ -n "$parent" ] || die 'usage: ticket.sh link child# parent#'
	gh issue comment "${child#\#}" --body "Parent: #${parent#\#}"
}

case "${1:-}" in
new)
	shift
	cmd_new "$@"
	;;
find)
	shift
	cmd_find "$@"
	;;
ls)
	shift
	cmd_ls "$@"
	;;
link)
	shift
	cmd_link "$@"
	;;
*) sed -n '2,10p' "$0" | sed 's/^# \{0,1\}//' ;;
esac
