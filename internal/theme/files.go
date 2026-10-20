package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// maxBaseDepth bounds a chain of themes built on each other, which also catches a cycle.
const maxBaseDepth = 8

// Dir is where the user's themes are: the themes folder beside noct's config file.
func Dir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "noct", "themes"), nil
}

// file is a theme file as read: its base and its colors by table and key.
type file struct {
	base   string
	colors map[Component]Color
}

func readFile(path string) (file, error) {
	var raw map[string]any
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return file{}, err
	}
	f := file{base: "noct", colors: map[Component]Color{}}
	for key, value := range raw {
		if key == "base" {
			base, ok := value.(string)
			if !ok {
				return file{}, fmt.Errorf("base is not a theme name")
			}
			f.base = base
			continue
		}
		table, ok := value.(map[string]any)
		if !ok {
			return file{}, fmt.Errorf("%s is not a table of colors", key)
		}
		for name, v := range table {
			c, ok := component(key, name)
			if !ok {
				return file{}, fmt.Errorf("%s.%s is not something a theme colors", key, name)
			}
			var text string
			switch v := v.(type) {
			case string:
				text = v
			case int64:
				text = strconv.FormatInt(v, 10) // a terminal color written without quotes
			default:
				return file{}, fmt.Errorf("%s.%s is not a color", key, name)
			}
			color, err := ParseColor(text)
			if err != nil {
				return file{}, fmt.Errorf("%s.%s: %w", key, name, err)
			}
			if color == "" && !FollowsBackground(c) {
				return file{}, fmt.Errorf("%s.%s needs a color: only backgrounds can be empty", key, name)
			}
			f.colors[c] = color
		}
	}
	return f, nil
}

func component(group, key string) (Component, bool) {
	for c, info := range Components {
		if info.Group == group && info.Key == key {
			return Component(c), true
		}
	}
	return 0, false
}

// LoadDir reads the themes in dir, by name. A theme that cannot be read is left out, with an
// error naming its file; a missing folder has no themes.
func LoadDir(dir string, builtins []Theme) ([]Theme, []error) {
	paths, _ := filepath.Glob(filepath.Join(dir, "*.toml"))
	if len(paths) == 0 {
		return nil, nil
	}
	slices.Sort(paths)
	files := map[string]file{}
	var errs []error
	var names []string
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".toml")
		if err := CheckName(name); err != nil {
			errs = append(errs, fmt.Errorf("theme %s: %w", filepath.Base(path), err))
			continue
		}
		f, err := readFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("theme %s: %w", filepath.Base(path), err))
			continue
		}
		files[name] = f
		names = append(names, name)
	}
	var resolve func(name string, depth int) (Theme, error)
	resolve = func(name string, depth int) (Theme, error) {
		if t, ok := Find(builtins, name); ok {
			return t, nil
		}
		f, ok := files[name]
		if !ok {
			return Theme{}, fmt.Errorf("there is no theme %s", name)
		}
		if depth == maxBaseDepth {
			return Theme{}, fmt.Errorf("its bases go round in a circle")
		}
		base, err := resolve(f.base, depth+1)
		if err != nil {
			return Theme{}, err
		}
		t := Theme{Name: name, Base: f.base, Colors: base.Colors}
		for c, color := range f.colors {
			t.Colors[c] = color
		}
		return t, nil
	}
	var themes []Theme
	for _, name := range names {
		t, err := resolve(name, 0)
		if err != nil {
			errs = append(errs, fmt.Errorf("theme %s.toml: %w", name, err))
			continue
		}
		themes = append(themes, t)
	}
	return themes, errs
}

// Save writes t to dir with only the colors that differ from base, through a temporary file so
// a theme being read is never half written.
func Save(dir string, t Theme, base Theme) error {
	var out strings.Builder
	fmt.Fprintf(&out, "base = %s\n", strconv.Quote(t.Base))
	group := ""
	for c, info := range Components {
		if t.Colors[c] == base.Colors[c] {
			continue
		}
		if info.Group != group {
			group = info.Group
			fmt.Fprintf(&out, "\n[%s]\n", group)
		}
		fmt.Fprintf(&out, "%s = %s\n", info.Key, strconv.Quote(string(t.Colors[c])))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, t.Name+".toml")
	if err := os.WriteFile(path+".tmp", []byte(out.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}
