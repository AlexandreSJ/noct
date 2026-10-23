package app

import (
	"errors"
	"io/fs"
	"os"
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/examples"
	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/script"
)

// setupStep is a page of the setup, in the order they come.
type setupStep int

const (
	setupStart setupStep = iota
	setupKeys
	setupThemes
	setupAnimations
	setupDemo
	setupMCP
	setupBinds
	setupSummary
)

// The first page's choices.
const (
	setupKeep    = "keep everything as it is"
	setupDefault = "start with the defaults"
	setupChoose  = "choose step by step"
)

// Animation presets: every moment as the defaults have it, a few short ones, or none.
const (
	animationsAll  = "all"
	animationsFew  = "few"
	animationsNone = "none"
)

var (
	setupPrefixes  = []string{"ctrl+space", "alt+space"}
	setupShortcuts = []string{"alt", "ctrl", "ctrl+alt", "off"}
	animationSets  = []string{animationsAll, animationsFew, animationsNone}
)

// setupWizard is the setup of noct (noct setup, and the first run): a page per step, each
// changing draft, which is saved only at the end.
type setupWizard struct {
	step     setupStep
	firstRun bool
	selected int
	draft    config.Config
	// themes are the themes offered, and installed which of them the picker shows.
	themes    []string
	installed map[string]bool
	animation string
	demo      bool
	// mcp and binds are what the system has, each marked when it is to be added.
	mcp       []mcpSetup
	mcpMarked []bool
	binds     []config.CustomBind
	bindsOn   []bool
}

// isFirstRun tells a first run from an upgrade: neither the config nor any session exists.
func isFirstRun(configPath, sessionsPath string) bool {
	_, configErr := os.Stat(configPath)
	_, sessionsErr := os.Stat(sessionsPath)
	return errors.Is(configErr, fs.ErrNotExist) && errors.Is(sessionsErr, fs.ErrNotExist)
}

// openSetupOnAttach opens the setup when the terminal asked for it (noct setup), or once on the
// first run.
func (a *App) openSetupOnAttach(asked bool) {
	if !asked && !a.firstRun {
		return
	}
	first := a.firstRun
	a.firstRun = false
	a.openSetup()
	a.setup.firstRun = first
}

func (a *App) openSetup() {
	w := setupWizard{draft: a.config, demo: true, animation: animationsAll, installed: map[string]bool{}}
	for _, t := range a.themeList() {
		w.themes = append(w.themes, t.Name)
		w.installed[t.Name] = len(a.config.Appearance.Themes) == 0 || slices.Contains(a.config.Appearance.Themes, t.Name)
	}
	for _, s := range mcpSetups(a.noctBinary()) {
		if s.file == "" && isInstalled(strings.Fields(s.text)[0]) {
			w.mcp, w.mcpMarked = append(w.mcp, s), append(w.mcpMarked, false)
		}
	}
	for _, b := range exampleBinds() {
		if program := strings.Fields(b.Command)[0]; isInstalled(program) && !slices.ContainsFunc(a.config.CustomBinds, func(c config.CustomBind) bool { return c.Key == b.Key }) {
			w.binds, w.bindsOn = append(w.binds, b), append(w.bindsOn, true)
		}
	}
	a.setup = w
	a.mode = setupMode
}

// exampleBinds are the binds the setup offers, for the programs the system has.
func exampleBinds() []config.CustomBind {
	return []config.CustomBind{
		{Key: "ctrl+alt+g", Name: "glow", Command: "glow {{.file}}"},
		{Key: "ctrl+alt+l", Name: "lazygit", Command: "lazygit"},
		{Key: "ctrl+alt+b", Name: "btop", Command: "btop"},
		{Key: "ctrl+alt+b", Name: "htop", Command: "htop"},
	}
}

func (w *setupWizard) startChoices() []string {
	if w.firstRun {
		return []string{setupDefault, setupChoose}
	}
	return []string{setupKeep, setupDefault, setupChoose}
}

func (w *setupWizard) themeNames() []string {
	return w.themes
}

// rows is how many rows the step has to select from.
func (w *setupWizard) rows() int {
	switch w.step {
	case setupStart:
		return len(w.startChoices())
	case setupKeys:
		return 2
	case setupThemes:
		return len(w.themes)
	case setupAnimations:
		return len(animationSets)
	case setupMCP:
		return len(w.mcp)
	case setupBinds:
		return len(w.binds)
	}
	return 1
}

func (a *App) handleSetupKey(key uv.KeyPressEvent) {
	w := &a.setup
	switch name := key.String(); name {
	case "up", "k":
		w.selected = (w.selected - 1 + max(w.rows(), 1)) % max(w.rows(), 1)
	case "down", "j":
		w.selected = (w.selected + 1) % max(w.rows(), 1)
	case "esc":
		a.setupBack()
	case "enter":
		a.setupEnter()
	case "s":
		if w.step != setupStart && w.step != setupSummary {
			a.skipSetupStep()
		}
	default:
		a.setupStepKey(name)
	}
	if w.step == setupThemes {
		a.previewSetupTheme()
	}
}

// setupStepKey runs the keys a step has of its own.
func (a *App) setupStepKey(name string) {
	w := &a.setup
	switch {
	case w.step == setupKeys && (name == "left" || name == "right" || name == "h" || name == "l"):
		w.changeKey(map[string]int{"left": -1, "h": -1, "right": 1, "l": 1}[name])
	case w.step == setupThemes && name == "space":
		theme := w.themes[w.selected]
		w.installed[theme] = !w.installed[theme]
	case w.step == setupThemes && name == "a":
		all := !slices.ContainsFunc(w.themes, func(t string) bool { return !w.installed[t] })
		for _, t := range w.themes {
			w.installed[t] = !all
		}
	case w.step == setupThemes && name == "m":
		w.draft.Appearance.Mode = map[bool]string{true: config.ModeDark, false: config.ModeLight}[w.draft.Appearance.Mode == config.ModeLight]
	case w.step == setupThemes && name == "b":
		w.draft.Appearance.Background = map[bool]string{true: config.BackgroundTerminal, false: config.BackgroundTheme}[w.draft.Appearance.Background == config.BackgroundTheme]
	case w.step == setupDemo && name == "space":
		w.demo = !w.demo
	case w.step == setupMCP && name == "space" && len(w.mcp) > 0:
		w.mcpMarked[w.selected] = !w.mcpMarked[w.selected]
	case w.step == setupBinds && name == "space" && len(w.binds) > 0:
		w.bindsOn[w.selected] = !w.bindsOn[w.selected]
	}
}

// changeKey moves the selected key setting to the next choice.
func (w *setupWizard) changeKey(step int) {
	if w.selected == 0 {
		prefix := cycle(setupPrefixes, prefixName(w.draft.Prefix), step)
		w.draft.Prefix = config.Prefix{Ctrl: prefix == "ctrl+space", Alt: prefix == "alt+space", Key: "space"}
		return
	}
	shortcut := cycle(setupShortcuts, shortcutName(w.draft.Shortcut), step)
	w.draft.Shortcut = config.Shortcut{Ctrl: strings.Contains(shortcut, "ctrl"), Alt: strings.Contains(shortcut, "alt")}
}

func prefixName(p config.Prefix) string {
	if p.Alt && !p.Ctrl && p.Key == "space" {
		return "alt+space"
	}
	if p.Ctrl && !p.Alt && !p.Shift && p.Key == "space" {
		return "ctrl+space"
	}
	return "your own"
}

func shortcutName(s config.Shortcut) string {
	switch {
	case s.Ctrl && s.Alt:
		return "ctrl+alt"
	case s.Ctrl:
		return "ctrl"
	case s.Alt:
		return "alt"
	}
	return "off"
}

// previewSetupTheme shows the selected theme on the whole screen while the themes are chosen.
func (a *App) previewSetupTheme() {
	w := &a.setup
	t := a.themeNamed(w.themes[w.selected]).In(w.draft.Appearance.Mode == config.ModeLight)
	a.palette = newPalette(t, w.draft.Appearance.Background == config.BackgroundTheme)
}

func (a *App) setupEnter() {
	w := &a.setup
	switch w.step {
	case setupStart:
		a.startSetup(w.startChoices()[w.selected])
		return
	case setupThemes:
		theme := w.themes[w.selected]
		w.installed[theme] = true
		w.draft.Appearance.Theme = theme
	case setupAnimations:
		w.animation = animationSets[w.selected]
		applyAnimationSet(&w.draft, w.animation)
	case setupSummary:
		a.finishSetup()
		return
	}
	a.goToSetupStep(w.step + 1)
}

func (a *App) startSetup(choice string) {
	switch choice {
	case setupKeep:
		a.closeSetup()
	case setupDefault:
		a.setup.draft = config.Default()
		a.saveSetupConfig()
		a.closeSetup()
	case setupChoose:
		a.goToSetupStep(setupKeys)
	}
}

// skipSetupStep leaves the step's settings as they were before the setup.
func (a *App) skipSetupStep() {
	w := &a.setup
	switch w.step {
	case setupKeys:
		w.draft.Prefix, w.draft.Shortcut = a.config.Prefix, a.config.Shortcut
	case setupThemes:
		w.draft.Appearance = a.config.Appearance
		for _, t := range w.themes {
			w.installed[t] = len(a.config.Appearance.Themes) == 0 || slices.Contains(a.config.Appearance.Themes, t)
		}
	case setupAnimations:
		w.draft.Animations, w.draft.AnimationTimes = a.config.Animations, a.config.AnimationTimes
		w.draft.Appearance.Animations = a.config.Appearance.Animations
	case setupDemo:
		w.demo = false
	case setupMCP:
		clear(w.mcpMarked)
	case setupBinds:
		clear(w.bindsOn)
	}
	a.goToSetupStep(w.step + 1)
}

func (a *App) setupBack() {
	if a.setup.step == setupStart {
		a.closeSetup()
		return
	}
	a.goToSetupStep(a.setup.step - 1)
}

func (a *App) goToSetupStep(step setupStep) {
	w := &a.setup
	w.step, w.selected = step, 0
	switch step {
	case setupThemes:
		w.selected = max(slices.Index(w.themes, w.draft.Appearance.Theme), 0)
		a.previewSetupTheme()
	case setupAnimations:
		w.selected = slices.Index(animationSets, w.animation)
	}
	if step != setupThemes {
		a.applyPalette()
	}
}

// applyAnimationSet gives every moment the preset's effect and time.
func applyAnimationSet(c *config.Config, set string) {
	defaults := config.Default()
	c.Animations, c.AnimationTimes = defaults.Animations, defaults.AnimationTimes
	c.Appearance.Animations = config.AnimationsOn
	switch set {
	case animationsNone:
		c.Appearance.Animations = config.AnimationsOff
	case animationsFew:
		for _, moment := range config.AnimationMoments {
			if moment != "hover" && moment != "popup" && moment != "activity" {
				c.Animations.SetEffect(moment, "none")
			}
		}
	}
}

// finishSetup saves what the steps chose, installs the demo and adds noct to the AI clients
// marked, each in a pane that shows how it went.
func (a *App) finishSetup() {
	w := &a.setup
	w.draft.Appearance.Themes = nil
	if slices.ContainsFunc(w.themes, func(t string) bool { return !w.installed[t] }) {
		for _, t := range w.themes {
			if w.installed[t] {
				w.draft.Appearance.Themes = append(w.draft.Appearance.Themes, t)
			}
		}
	}
	for i, b := range w.binds {
		if w.bindsOn[i] {
			w.draft.CustomBinds = append(w.draft.CustomBinds, b)
		}
	}
	a.saveSetupConfig()
	if w.demo {
		if _, err := examples.InstallDemo(a.scriptsDir(), flow.Dir()); err != nil {
			a.statusMessage = err.Error()
		}
		a.reloadScripts()
	}
	commands := a.markedMCPCommands()
	a.closeSetup()
	for _, s := range commands {
		a.startRun(s, nil, false)
	}
}

func (a *App) markedMCPCommands() []script.Script {
	var commands []script.Script
	for i, s := range a.setup.mcp {
		if a.setup.mcpMarked[i] {
			commands = append(commands, script.Script{Name: "add noct to " + s.client, Run: s.text})
		}
	}
	return commands
}

func (a *App) saveSetupConfig() {
	a.applyConfig(a.setup.draft)
	a.keymap = newKeymap(a.config.Keys)
}

func (a *App) closeSetup() {
	a.mode = normalMode
	a.applyPalette()
}
