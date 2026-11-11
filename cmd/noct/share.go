package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/share"
	"github.com/AlexandreSJ/noct/internal/theme"
)

// noct export prints a theme, the config or the keys as one line that noct import runs on another
// machine, and as a QR code of that line on a terminal; noct import reads it back, or a theme
// file of kitty, Ghostty or base16, and asks before it changes anything.

// shareIO is where the commands read answers and write, and whether a person is at both ends.
type shareIO struct {
	in          io.Reader
	out         io.Writer
	interactive bool
	configPath  string
	themesDir   string
}

func terminalShareIO() (shareIO, error) {
	path, err := config.Path()
	if err != nil {
		return shareIO{}, err
	}
	themes, err := theme.Dir()
	if err != nil {
		return shareIO{}, err
	}
	return shareIO{in: os.Stdin, out: os.Stdout, interactive: isTerminal(os.Stdin) && isTerminal(os.Stdout), configPath: path, themesDir: themes}, nil
}

const exportUsage = "usage: noct export theme [name] | config | keys [--no-qr]"

func exportCommand(args []string, sio shareIO) error {
	set := flag.NewFlagSet("export", flag.ContinueOnError)
	noQR := set.Bool("no-qr", false, "only the line")
	if err := set.Parse(moveFlagsFirst(args)); err != nil {
		return err
	}
	cfg, err := config.Load(sio.configPath)
	if err != nil {
		return err
	}
	var p share.Payload
	switch what := set.Args(); {
	case len(what) == 0:
		return errors.New(exportUsage)
	case what[0] == share.KindTheme && len(what) <= 2:
		name := cfg.Appearance.Theme
		if len(what) == 2 {
			name = what[1]
		}
		p, err = exportTheme(name, sio.themesDir)
	case what[0] == share.KindConfig && len(what) == 1:
		p.Kind = share.KindConfig
		p.Body, err = changedSettings(cfg)
	case (what[0] == share.KindKeys || what[0] == "shortcuts") && len(what) == 1:
		p.Kind = share.KindKeys
		p.Body, err = encodeTOML(keysOf(cfg))
	default:
		return errors.New(exportUsage)
	}
	if err != nil {
		return err
	}
	line := share.Command(share.Encode(p))
	if sio.interactive && !*noQR {
		if drawn, err := share.QR(line); err == nil {
			fmt.Fprint(sio.out, drawn)
		} else {
			fmt.Fprintln(sio.out, err)
		}
		fmt.Fprintln(sio.out, "Scan the code, or copy the line below, and run it on the other machine:")
	}
	fmt.Fprintln(sio.out, line)
	return nil
}

// exportTheme is the theme as a code: on its base when that is a theme noct comes with, else on
// noct's, so the code needs nothing the other machine may not have.
func exportTheme(name, dir string) (share.Payload, error) {
	builtins := theme.Builtins()
	mine, _ := theme.LoadDir(dir, builtins)
	t, ok := theme.Find(append(builtins, mine...), name)
	if !ok {
		return share.Payload{}, fmt.Errorf("there is no theme %s", name)
	}
	base, ok := theme.Find(builtins, t.Base)
	if t.Builtin {
		base, ok = t, true
		t.Base = t.Name
	}
	if !ok {
		base, t.Base = builtins[0], builtins[0].Name
	}
	return share.Payload{Kind: share.KindTheme, Name: t.Name, Body: theme.Encode(t, base)}, nil
}

// keys are the parts of the config a keys code carries.
type keys struct {
	Keys        map[string]map[string][]string `toml:"keys,omitempty"`
	CustomBinds []config.CustomBind            `toml:"custom_binds,omitempty"`
}

func keysOf(cfg config.Config) keys {
	return keys{Keys: cfg.Keys, CustomBinds: cfg.CustomBinds}
}

func encodeTOML(v any) ([]byte, error) {
	var out bytes.Buffer
	err := toml.NewEncoder(&out).Encode(v)
	return out.Bytes(), err
}

// changedSettings are the settings of cfg that differ from the defaults, without the keys, which
// have their own code.
func changedSettings(cfg config.Config) ([]byte, error) {
	cfg.Keys, cfg.CustomBinds = nil, nil
	mine, err := settings(cfg)
	if err != nil {
		return nil, err
	}
	defaults, err := settings(config.Default())
	if err != nil {
		return nil, err
	}
	tables := map[string]map[string]any{}
	top := map[string]any{}
	for path, value := range mine {
		if fmt.Sprint(defaults[path]) == fmt.Sprint(value) {
			continue
		}
		if table, key, ok := strings.Cut(path, "."); ok {
			if tables[table] == nil {
				tables[table] = map[string]any{}
			}
			tables[table][key] = value
		} else {
			top[path] = value
		}
	}
	for table, values := range tables {
		top[table] = values
	}
	return encodeTOML(top)
}

// settings are a config's settings by path ("logs.keep_days") and value, two levels deep as the
// config is; deeper tables (the MCP policy) are one value.
func settings(cfg config.Config) (map[string]any, error) {
	text, err := encodeTOML(cfg)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if _, err := toml.Decode(string(text), &raw); err != nil {
		return nil, err
	}
	flat := map[string]any{}
	for key, value := range raw {
		table, ok := value.(map[string]any)
		if !ok || key == "keys" {
			flat[key] = value
			continue
		}
		for inner, v := range table {
			flat[key+"."+inner] = v
		}
	}
	return flat, nil
}

const importUsage = "usage: noct import <code> | noct import theme <file> [--name name] [--yes]"

func importCommand(args []string, sio shareIO) error {
	set := flag.NewFlagSet("import", flag.ContinueOnError)
	yes := set.Bool("yes", false, "change without asking")
	name := set.String("name", "", "the name of the imported theme")
	if err := set.Parse(moveFlagsFirst(args)); err != nil {
		return err
	}
	ask := func(question string) bool { return *yes || confirm(sio, question) }
	switch what := set.Args(); {
	case len(what) == 2 && what[0] == share.KindTheme:
		return importThemeFile(what[1], *name, sio, ask)
	case len(what) == 1:
		p, err := share.Decode(what[0])
		if err != nil {
			return err
		}
		switch p.Kind {
		case share.KindTheme:
			t, err := theme.Parse(cmpOr(*name, p.Name), p.Body, theme.Builtins())
			if err != nil {
				return fmt.Errorf("the theme in the code: %w", err)
			}
			return saveImportedTheme(t, sio, ask)
		case share.KindConfig:
			return importSettings(p.Body, sio, ask)
		case share.KindKeys:
			return importKeys(p.Body, sio, ask)
		}
	}
	return errors.New(importUsage)
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// importThemeFile reads a noct theme file, or a kitty, Ghostty or base16 one, named after the
// file unless a name is given.
func importThemeFile(path, name string, sio shareIO, ask func(string) bool) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if name == "" {
		name = themeNameOf(path)
	}
	t, err := theme.Parse(name, content, theme.Builtins())
	if err != nil {
		var foreignErr error
		if t, foreignErr = theme.Foreign(name, content); foreignErr != nil {
			return fmt.Errorf("%s: %w", path, foreignErr)
		}
	}
	return saveImportedTheme(t, sio, ask)
}

var notInName = regexp.MustCompile(`[^a-z0-9_-]+`)

// themeNameOf names a theme after its file: "Tokyo Night.conf" is tokyo-night.
func themeNameOf(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return strings.Trim(notInName.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

func saveImportedTheme(t theme.Theme, sio shareIO, ask func(string) bool) error {
	if err := theme.CheckName(t.Name); err != nil {
		return fmt.Errorf("%w; give another with --name", err)
	}
	builtins := theme.Builtins()
	if _, builtin := theme.Find(builtins, t.Name); builtin {
		return fmt.Errorf("noct comes with a theme %s; give the import another name with --name", t.Name)
	}
	path := filepath.Join(sio.themesDir, t.Name+".toml")
	if _, err := os.Stat(path); err == nil && !ask(fmt.Sprintf("Replace your theme %s?", t.Name)) {
		return errors.New("nothing changed")
	}
	base, _ := theme.Find(builtins, t.Base)
	if err := theme.Save(sio.themesDir, t, base); err != nil {
		return err
	}
	fmt.Fprintf(sio.out, "Theme %s saved in %s. Choose it in the config (Appearance → theme); with background on theme, programs get its 16 colors too.\n", t.Name, path)
	return nil
}

// importSettings shows the settings the code changes, which keep the keys, and saves them once
// the user agrees.
func importSettings(body []byte, sio shareIO, ask func(string) bool) error {
	current, err := config.Load(sio.configPath)
	if err != nil {
		return err
	}
	next, err := config.Overlay(current, body)
	if err != nil {
		return fmt.Errorf("the settings in the code: %w", err)
	}
	next.Keys, next.CustomBinds = current.Keys, current.CustomBinds
	before, _ := settings(current)
	after, _ := settings(next)
	var changes []string
	for _, path := range slices.Sorted(maps.Keys(after)) {
		if old, now := fmt.Sprint(before[path]), fmt.Sprint(after[path]); old != now {
			changes = append(changes, fmt.Sprintf("  %s: %s → %s", path, old, now))
		}
	}
	if len(changes) == 0 {
		fmt.Fprintln(sio.out, "Your settings are already these.")
		return nil
	}
	fmt.Fprintln(sio.out, "The code changes these settings:")
	fmt.Fprintln(sio.out, strings.Join(changes, "\n"))
	if slices.ContainsFunc(changes, func(c string) bool { return strings.Contains(c, "editor:") || strings.Contains(c, "file_manager:") }) {
		fmt.Fprintln(sio.out, "The editor and file manager are commands noct runs: check them before you agree.")
	}
	return saveConfig(next, sio, ask("Change them?"))
}

// importKeys shows the keys and binds the code brings, each command a bind runs in full, and
// puts them in place of the user's once the user agrees.
func importKeys(body []byte, sio shareIO, ask func(string) bool) error {
	var k keys
	if _, err := toml.Decode(string(body), &k); err != nil {
		return fmt.Errorf("the keys in the code: %w", err)
	}
	current, err := config.Load(sio.configPath)
	if err != nil {
		return err
	}
	fmt.Fprintf(sio.out, "The code changes the keys of %d actions", countActions(k.Keys))
	if len(k.CustomBinds) == 0 {
		fmt.Fprintln(sio.out, " and has no binds of its own.")
	} else {
		fmt.Fprintln(sio.out, " and brings these binds, which run what they say when pressed:")
	}
	for _, b := range k.CustomBinds {
		kind, what, err := b.Target()
		if err != nil {
			return fmt.Errorf("the keys in the code: %w", err)
		}
		fmt.Fprintf(sio.out, "  %s  %s %s\n", b.Key, kind, what)
	}
	current.Keys, current.CustomBinds = k.Keys, k.CustomBinds
	return saveConfig(current, sio, ask("Put them in place of yours?"))
}

func countActions(keys map[string]map[string][]string) int {
	n := 0
	for _, actions := range keys {
		n += len(actions)
	}
	return n
}

func saveConfig(cfg config.Config, sio shareIO, agreed bool) error {
	if !agreed {
		return errors.New("nothing changed")
	}
	if err := config.Save(sio.configPath, cfg); err != nil {
		return err
	}
	fmt.Fprintln(sio.out, "Saved. A running noct reads it when it starts again: noct quit, then noct.")
	return nil
}

// confirm asks a yes or no question; without a person to answer, or for anything but yes, it is no.
func confirm(sio shareIO, question string) bool {
	if !sio.interactive {
		fmt.Fprintln(sio.out, question+" Run it again with --yes to agree without being asked.")
		return false
	}
	fmt.Fprint(sio.out, question+" [y/N] ")
	answer, _ := bufio.NewReader(sio.in).ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

// moveFlagsFirst lets flags come after the words, as in noct import theme x.conf --name x.
func moveFlagsFirst(args []string) []string {
	var flags, words []string
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--name" && i+1 < len(args):
			flags = append(flags, arg, args[i+1])
			i++
		case strings.HasPrefix(arg, "-"):
			flags = append(flags, arg)
		default:
			words = append(words, arg)
		}
	}
	return append(flags, words...)
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
