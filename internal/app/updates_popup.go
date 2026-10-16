package app

import (
	"image"
	"strings"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/update"
)

// The updates popup (prefix+c) says whether a newer noct is out and shows the changelog of each
// release, the newest first; its keys check again, install the newest and restart into it.

const updatesPanelWidth = 78

// openUpdates shows the updates popup, checking first when nothing was checked yet.
func (a *App) openUpdates() {
	a.updates.first, a.updates.copied = 0, false
	if !a.updates.checked {
		a.checkForUpdates()
	}
	a.mode = updatesMode
}

func (a *App) handleUpdatesKey(key uv.KeyPressEvent) {
	switch key.String() {
	case "esc", "q":
		a.mode = normalMode
	case "up", "k":
		a.updates.first = max(a.updates.first-1, 0)
	case "down", "j":
		a.updates.first++
	case "pgup", "pgdown", "home", "end":
		last := max(a.updates.lines-a.updates.visible, 0) // the first line that shows the end
		a.updates.first, _ = listJump(key.String(), a.updates.first, last+1, a.updates.visible)
	case "c":
		a.checkForUpdates()
	case "u":
		a.installUpdate()
	case "y":
		if a.updates.manual != "" {
			a.copyToClipboard(a.updates.manual)
			a.updates.copied = true
		}
	case "r":
		a.askToRestart()
		a.confirm.cancel = func() { a.mode = updatesMode }
	}
}

// updatesStatus is what the popup says first: what is going on, or what the user can do.
func (a *App) updatesStatus() []styledLine {
	pal := &a.palette
	newest, hasNewer := a.newestRelease()
	switch {
	case a.updates.installing:
		return []styledLine{{"installing " + newest.Version + "…", pal.panelMuted}}
	case a.updates.installed != "":
		return []styledLine{{a.updates.installed + " is installed: " + a.hint("updates", "{r}") + " restarts noct to run it; the programs in panes stop", pal.focusedTitle}}
	case a.updates.manual != "":
		copied := a.hint("updates", "{y} copies it")
		if a.updates.copied {
			copied = "copied to the clipboard"
		}
		return []styledLine{
			{"you cannot write where noct is installed; run this in a terminal, then restart noct:", pal.focusedTitle},
			{a.updates.manual, pal.panelText},
			{copied, pal.panelMuted},
		}
	case a.updates.checking:
		return []styledLine{{"checking for new versions…", pal.panelMuted}}
	case a.updates.problem != "":
		return []styledLine{{"could not check: " + a.updates.problem, pal.status}}
	case !update.IsRelease(a.version):
		return []styledLine{{"this is a development build; it updates from its source, not from the releases", pal.panelMuted}}
	case hasNewer:
		return []styledLine{{newest.Version + " is out: " + a.hint("updates", "{u}") + " installs it", pal.focusedTitle}}
	case a.updates.checked:
		return []styledLine{{"noct " + a.version + " is the newest release", pal.panelMuted}}
	}
	return []styledLine{{"not checked yet: " + a.hint("updates", "{c}") + " checks", pal.panelMuted}}
}

// changelogLines are the releases and their notes, the newest first, marking the ones newer
// than this noct and this noct's own.
func (a *App) changelogLines() []styledLine {
	pal := &a.palette
	if len(a.updates.releases) == 0 {
		if a.updates.checked {
			return []styledLine{{"no release is published yet", pal.panelMuted}}
		}
		return nil
	}
	var lines []styledLine
	for _, r := range a.updates.releases {
		heading, style := r.Version, pal.panelHeading
		if !r.Published.IsZero() {
			heading += " · " + r.Published.Format("Jan 2, 2006")
		}
		switch order := update.Compare(r.Version, a.version); {
		case order == 0:
			heading += " · this version"
		case order > 0 && update.IsRelease(a.version):
			heading, style = heading+" · new", pal.focusedTitle
		}
		lines = append(lines, styledLine{heading, style})
		for _, note := range strings.Split(strings.TrimSpace(strings.ReplaceAll(r.Notes, "\r", "")), "\n") {
			lines = append(lines, styledLine{note, pal.panelText})
		}
		lines = append(lines, styledLine{"", pal.panelText})
	}
	return lines[:len(lines)-1]
}

// updatesHints are the popup's keys that do something now.
func (a *App) updatesHints() string {
	hints := ""
	if a.canCheck() {
		hints = "{c} check  "
	}
	if _, ok := a.newestRelease(); ok && a.updates.installed == "" && !a.updates.installing {
		hints += "{u} update  "
	}
	if a.updates.installed != "" {
		hints += "{r} restart  "
	}
	if a.updates.manual != "" {
		hints += "{y} copy  "
	}
	return a.hint("updates", hints+"{up}{down} scroll  esc")
}

func (a *App) drawUpdates(screen uv.Screen) {
	area := image.Rect(0, 0, a.size.X, a.size.Y)
	width := min(updatesPanelWidth, a.size.X-4)
	source := a.updatesStatus()
	if !a.updates.lastChecked.IsZero() && !a.updates.checking {
		source = append(source, styledLine{"checked " + lastUsed(a.updates.lastChecked, time.Now(), false), a.palette.panelMuted})
	}
	source = append(source, styledLine{"", a.palette.panelText})
	source = append(source, a.changelogLines()...)
	var lines []styledLine
	for _, line := range source {
		for _, part := range strings.Split(ansi.Wrap(line.text, width-6, ""), "\n") {
			lines = append(lines, styledLine{part, line.style})
		}
	}
	inner := a.drawPanel(screen, centeredRect(area, width, panelHeight(len(lines)+2)), "updates · noct "+a.version)
	visible := max(inner.Dy()-2, 1)
	a.updates.first = min(a.updates.first, max(len(lines)-visible, 0))
	a.updates.visible, a.updates.lines = visible, len(lines)
	for i := a.updates.first; i < len(lines) && i-a.updates.first < visible; i++ {
		drawText(screen, inner.Min.X+1, inner.Min.Y+i-a.updates.first, inner.Max.X-1, lines[i].text, lines[i].style)
	}
	drawText(screen, inner.Min.X+1, inner.Max.Y-1, inner.Max.X, a.updatesHints(), a.palette.panelMuted)
}
