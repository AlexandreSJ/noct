#!/bin/sh
# Records the README's GIFs with VHS (https://github.com/charmbracelet/vhs): each tape in this
# folder, or only the ones given, into docs/assets/. noct runs in a home of its own with the demo
# scripts and flows, so your workspace, config and shell setup are not touched.
#
#   docs/demo/record.sh [tape ...]      needs vhs, ttyd and ffmpeg in PATH
set -eu

demo=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$demo/../.." && pwd)

# A short root: the daemon's socket path must fit in 108 bytes.
root=$(mktemp -d /tmp/noct-demo.XXXX)
cleanup() {
	noct quit >/dev/null 2>&1 || true
	rm -rf "$root"
}
trap cleanup EXIT

mkdir -p "$root/bin" "$root/home" "$root/run" "$root/state" "$root/config" "$root/data/noct" "$root/work"
chmod 700 "$root/run"
cp -r "$demo/scripts" "$demo/flows" "$root/data/noct/"
(cd "$repo" && go build -o "$root/bin/noct" ./cmd/noct)

# Logs of the last four weeks, some days busier than others, for the logs calendar's colors.
i=1
while [ $i -le 28 ]; do
	day=$(date -d "-$i day" +%Y-%m-%d) # GNU date
	mkdir -p "$root/state/noct/logs/$day"
	n=$(((i * 7 + i / 5) % 9))
	[ $((i % 7)) -eq 3 ] && n=0 # a quiet day each week
	j=0
	while [ $j -lt $n ]; do
		printf 'GET https://staging.shop.test/health\n200 OK\n' >"$root/state/noct/logs/$day/$(printf '%02d%02d00' $((9 + j)) $((j * 7 % 60)))-api-health.log"
		j=$((j + 1))
	done
	i=$((i + 1))
done

cat >"$root/bashrc" <<'EOF'
PS1='\[\e[38;2;157;124;216m\]❯\[\e[0m\] '
unset PROMPT_COMMAND
HISTFILE=/dev/null
cd "$NOCT_DEMO_WORK"
EOF
# The panes' shell: bash with the prompt above when interactive, plain bash for the -c of a run.
# It is named bash so the panes' titles read bash.
mkdir -p "$root/shell"
cat >"$root/shell/bash" <<EOF
#!/bin/sh
[ \$# -gt 0 ] && exec $(command -v bash) "\$@"
exec $(command -v bash) --rcfile "$root/bashrc" -i
EOF
chmod +x "$root/shell/bash"

export HOME="$root/home" XDG_RUNTIME_DIR="$root/run" XDG_STATE_HOME="$root/state"
export XDG_CONFIG_HOME="$root/config" XDG_DATA_HOME="$root/data" NOCT_DEMO_WORK="$root/work"
export SHELL="$root/shell/bash" PATH="$root/bin:$PATH" LANG=C.UTF-8 LC_ALL=C.UTF-8

cd "$demo"
if [ $# -eq 0 ]; then
	set -- *.tape
fi
for tape in "$@"; do
	echo "recording $tape"
	vhs "$tape"
	noct quit >/dev/null 2>&1 || true
done
