package app

import (
	"image"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/layout"
)

// Ctrl+click on a terminal opens what is under the mouse: a link the program set (OSC 8), a URL
// in the text, even one a long line wraps, or the path of a file or folder that exists, with the
// system's opener. Over ssh, it is copied instead, since opening it would happen on the far side.

var urlPattern = regexp.MustCompile(`(?:https?|ftp|file)://[^\s<>"'` + "`" + `]+`)

// maxLinkRows bounds how many wrapped rows one link is read over.
const maxLinkRows = 8

// openLinkAt opens what is under point in pane id, telling whether there was something.
func (a *App) openLinkAt(t *tab, id layout.PaneID, point image.Point) bool {
	frame, ok := t.frameOf(id, a.workspaceArea())
	if !ok {
		return false
	}
	content := contentRect(frame)
	screen := uv.NewScreenBuffer(a.size.X, a.size.Y)
	v := t.views[id]
	if scrolled, ok := v.(scrollable); ok && a.mode == scrollMode && id == t.focused {
		scrolled.DrawScrolled(screen, content, a.scrollOffset)
	} else {
		v.Draw(screen, content)
	}
	target, ok := linkIn(screen, content, point, a.paneDir(t, id))
	if !ok {
		return false
	}
	safe, reason := safeTarget(target)
	if reason != "" {
		a.statusMessage = "not opened: " + reason
		return true
	}
	a.openTarget(safe)
	return true
}

// inertFiles are the kinds of file the opener only shows: text, documents, images and media.
// Anything else may be a program or a launcher, which opening would run, even without the
// executable bit or an extension the system reads it by.
var inertFiles = map[string]bool{}

func init() {
	for _, ext := range strings.Fields(`.txt .md .markdown .rst .log .csv .tsv .json .yaml .yml .toml .ini .conf .xml
		.html .htm .css .pdf .png .jpg .jpeg .gif .svg .webp .bmp .ico .mp3 .wav .ogg .flac .mp4 .webm .mkv .mov
		.go .mod .sum .py .rb .rs .c .h .cc .cpp .hpp .java .kt .ts .tsx .jsx .lua .sql .diff .patch .proto`) {
		inertFiles[ext] = true
	}
}

// safeTarget is what the opener may get for target, with links of files followed, or why not:
// any program can print a link, and opening a program or a launcher would run it.
func safeTarget(target string) (string, string) {
	if strings.HasPrefix(target, "-") {
		return "", "it starts with -"
	}
	path := target
	if u, err := url.Parse(target); err == nil && u.Scheme != "" && !filepath.IsAbs(target) {
		switch u.Scheme {
		case "http", "https", "ftp", "mailto":
			return target, ""
		case "file":
			path = u.Path
		default:
			return "", u.Scheme + " links are not opened"
		}
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", path + " does not exist"
	}
	info, err := os.Stat(resolved)
	switch {
	case err != nil:
		return "", path + " does not exist"
	case info.IsDir():
		return resolved, ""
	case !info.Mode().IsRegular() || info.Mode()&0o111 != 0:
		return "", path + " is a program"
	case !inertFiles[strings.ToLower(filepath.Ext(resolved))]:
		return "", path + " is not a kind of file noct opens: it could be a program"
	}
	return resolved, ""
}

// paneDir is the folder a pane's relative paths start from.
func (a *App) paneDir(t *tab, id layout.PaneID) string {
	if d, ok := t.views[id].(directoryTeller); ok && d.Dir() != "" {
		return strings.TrimSuffix(d.Dir(), " (deleted)")
	}
	return a.workDir
}

// linkIn is the link, URL or existing path under point on screen, in area.
func linkIn(screen uv.Screen, area image.Rectangle, point image.Point, dir string) (string, bool) {
	if c := screen.CellAt(point.X, point.Y); c != nil && c.Link.URL != "" {
		return c.Link.URL, true
	}
	text, at := logicalLine(screen, area, point)
	if at < 0 {
		return "", false
	}
	for _, span := range urlPattern.FindAllStringIndex(string(text), -1) {
		start, end := len([]rune(string(text)[:span[0]])), len([]rune(string(text)[:span[1]]))
		url := trimLink(text[start:end])
		if at >= start && at < start+len(url) {
			return string(url), true
		}
	}
	return existingPath(wordAt(text, at), dir)
}

// logicalLine is the text of the row at point joined with the rows a long line wrapped onto,
// as far as each row is full, and where point is in it; -1 off the text.
func logicalLine(screen uv.Screen, area image.Rectangle, point image.Point) ([]rune, int) {
	if !point.In(area) {
		return nil, -1
	}
	full := func(y int) bool {
		c := screen.CellAt(area.Max.X-1, y)
		if c != nil && c.Content == "" && area.Max.X-2 >= area.Min.X {
			c = screen.CellAt(area.Max.X-2, y) // a wide character at the end
		}
		return c != nil && strings.TrimSpace(c.Content) != ""
	}
	first := point.Y
	for first > area.Min.Y && point.Y-first < maxLinkRows && full(first-1) {
		first--
	}
	last := point.Y
	for last < area.Max.Y-1 && last-first < maxLinkRows && full(last) {
		last++
	}
	var text []rune
	at := -1
	for y := first; y <= last; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			c := screen.CellAt(x, y)
			if c != nil && c.Content == "" {
				continue // the second cell of a wide character
			}
			if x == point.X && y == point.Y {
				at = len(text)
			}
			content := " "
			if c != nil && c.Content != "" {
				content = c.Content
			}
			text = append(text, []rune(content)...)
		}
	}
	return text, at
}

// trimLink leaves out what ends a sentence after a link: punctuation, and a closing bracket the
// link did not open.
func trimLink(link []rune) []rune {
	for len(link) > 0 {
		last := link[len(link)-1]
		open := map[rune]rune{')': '(', ']': '[', '}': '{'}[last]
		switch {
		case strings.ContainsRune(".,;:!?'\"", last):
		case open != 0 && strings.Count(string(link), string(open)) < strings.Count(string(link), string(last)):
		default:
			return link
		}
		link = link[:len(link)-1]
	}
	return link
}

// wordAt is the run of text around at without spaces or the quotes and brackets around a path.
func wordAt(text []rune, at int) string {
	if at < 0 || at >= len(text) || unicode.IsSpace(text[at]) {
		return ""
	}
	bound := func(r rune) bool { return unicode.IsSpace(r) || strings.ContainsRune("\"'`<>()[]{}", r) }
	start, end := at, at
	for start > 0 && !bound(text[start-1]) {
		start--
	}
	for end < len(text) && !bound(text[end]) {
		end++
	}
	return string(text[start:end])
}

var lineSuffix = regexp.MustCompile(`(:\d+){1,2}$`)

// existingPath is word as a path that exists, from dir when relative, without the :line:column
// compilers and grep put after it.
func existingPath(word, dir string) (string, bool) {
	word = strings.TrimRight(word, ".,;:!?")
	if word == "" {
		return "", false
	}
	for _, candidate := range []string{word, lineSuffix.ReplaceAllString(word, "")} {
		path := candidate
		if rest, ok := strings.CutPrefix(path, "~/"); ok {
			home, err := os.UserHomeDir()
			if err != nil {
				continue
			}
			path = filepath.Join(home, rest)
		} else if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}

// openTarget opens a URL or path with the system's opener, or copies it when the terminal is far
// away or there is no opener.
func (a *App) openTarget(target string) {
	if a.client != nil && envValue(a.client.hello.Env, "SSH_CONNECTION") != "" || a.openURL == nil || !a.openURL(target) {
		a.copyToClipboard(target)
		a.notice = "copied " + target
		return
	}
	a.notice = "opened " + target
}

// systemOpen opens target with xdg-open, or open on macOS, telling whether it could start.
func systemOpen(target string) bool {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	path, err := exec.LookPath(opener)
	if err != nil {
		return false
	}
	cmd := exec.Command(path, target)
	if err := cmd.Start(); err != nil {
		return false
	}
	go func() { _ = cmd.Wait() }() // only reaps it
	return true
}
