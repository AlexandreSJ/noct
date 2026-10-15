package app

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/logstore"
)

// The log limits step through choices with the arrows, and take any value typed by hand: enter,
// or a digit, opens the value for typing; enter saves it and esc gives it up.

// Bounds of a typed log limit, so a slip of the keyboard cannot overflow it.
const (
	maxKeepDays  = 36500
	maxLogSizeKB = 1 << 30 // 1 TB
)

func isNumberRow(row configRow) bool {
	return row == logsKeepRow || row == logsSizeRow
}

// startsNumberEdit tells whether key opens the selected limit for typing: enter keeps the value
// to change it, a digit starts it over.
func (a *App) startsNumberEdit(key uv.KeyPressEvent) bool {
	row := a.configPanel.selected
	if !isNumberRow(row) {
		return false
	}
	if key.String() == "enter" {
		field := newTextField(a.numberText(row))
		a.configPanel.number = &field
		return true
	}
	if isPlainText(key) && unicode.IsDigit([]rune(key.Text)[0]) {
		field := newTextField(key.Text)
		a.configPanel.number = &field
		return true
	}
	return false
}

// numberText is a limit as typing starts from it: days, or a size in the largest whole unit.
func (a *App) numberText(row configRow) string {
	if row == logsKeepRow {
		return strconv.Itoa(a.config.Logs.KeepDays)
	}
	return sizeText(a.config.Logs.MaxSizeKB)
}

func sizeText(kb int) string {
	switch {
	case kb%(1<<20) == 0:
		return fmt.Sprintf("%d GB", kb>>20)
	case kb%(1<<10) == 0:
		return fmt.Sprintf("%d MB", kb>>10)
	}
	return fmt.Sprintf("%d KB", kb)
}

// typeNumber edits the limit being typed: enter saves, esc gives up, and moving away gives up too.
func (a *App) typeNumber(key uv.KeyPressEvent) {
	switch key.String() {
	case "esc":
		a.cancelNumberEdit()
	case "enter":
		a.saveNumber()
	case "up":
		a.cancelNumberEdit()
		a.moveConfigSelection(-1)
	case "down":
		a.cancelNumberEdit()
		a.moveConfigSelection(1)
	case "pgup", "pgdown":
		a.cancelNumberEdit()
		a.jumpConfigSelection(key.String())
	default:
		a.typeKey(key)
	}
}

func (a *App) cancelNumberEdit() {
	a.configPanel.number, a.configPanel.numberProblem = nil, ""
}

// saveNumber applies the typed limit; one that does not read stays open with the problem below it.
func (a *App) saveNumber() {
	text := a.configPanel.number.String()
	next := a.config
	var err error
	if a.configPanel.selected == logsKeepRow {
		next.Logs.KeepDays, err = parseKeepDays(text)
	} else {
		next.Logs.MaxSizeKB, err = parseLogSize(text)
	}
	if err != nil {
		a.configPanel.numberProblem = err.Error()
		return
	}
	a.applyLogLimits(next, a.cancelNumberEdit) // a no keeps the typed value open
}

// applyLogLimits puts next into effect and prunes the logs to its limits, then runs done. When
// the limits delete logs, it asks first, saying how many; a no changes nothing. The config panel
// shows again either way.
func (a *App) applyLogLimits(next config.Config, done func()) {
	apply := func() {
		a.mode = configMode
		a.applyConfig(next)
		a.pruneLogs()
		a.refreshLogUsage()
		if done != nil {
			done()
		}
	}
	limits := next.Logs
	expired, err := logstore.Expired(logstore.LogsDir(), limits.KeepDays, limits.MaxSizeBytes(), time.Now())
	if err != nil || len(expired) == 0 {
		apply() // logs that cannot be listed cannot be counted either; pruning reports the problem
		return
	}
	var size int64
	for _, e := range expired {
		size += e.Size
	}
	a.askToConfirm(fmt.Sprintf("delete %s (%s)?", plural(len(expired), "log"), logstore.PrettySize(size)), apply)
	a.confirm.cancel = func() { a.mode = configMode }
}

// refreshLogUsage reads how many logs there are and how much they take, for the bar under the
// limits; it is read when the panel opens and after the limits prune, not on every frame.
func (a *App) refreshLogUsage() {
	entries, err := logstore.List(logstore.LogsDir())
	if err != nil {
		a.configPanel.logUsage = nil // no bar rather than a wrong one
		return
	}
	usage := logUsage{count: len(entries)}
	for _, e := range entries {
		usage.size += e.Size
	}
	a.configPanel.logUsage = &usage
}

// logUsage is what the logs take on disk: how many and how many bytes.
type logUsage struct {
	count int
	size  int64
}

// logUsageLine is the bar of how much of the size limit the logs take, as the logs view shows it.
func (a *App) logUsageLine() []configLine {
	usage := a.configPanel.logUsage
	if usage == nil {
		return nil
	}
	limit := a.config.Logs.MaxSizeBytes()
	fraction := float64(usage.size) / float64(max(limit, 1))
	label := fmt.Sprintf("%.0f%% · %s of %s · %s", math.Min(fraction, 1)*100, logstore.PrettySize(usage.size), sizeText(a.config.Logs.MaxSizeKB), plural(usage.count, "log"))
	return []configLine{{label: label, note: true, bar: true, fraction: fraction}}
}

var keepDaysPattern = regexp.MustCompile(`^(\d+)\s*(d|days?)?$`)

// parseKeepDays reads "99", "99d" or "99 days".
func parseKeepDays(text string) (int, error) {
	match := keepDaysPattern.FindStringSubmatch(strings.ToLower(strings.TrimSpace(text)))
	if match == nil {
		return 0, errors.New("type a number of days, such as 99")
	}
	days, err := strconv.Atoi(match[1])
	if err != nil || days < 1 || days > maxKeepDays {
		return 0, fmt.Errorf("days go from 1 to %d", maxKeepDays)
	}
	return days, nil
}

var logSizePattern = regexp.MustCompile(`^(\d+(?:[.,]\d+)?)\s*(k|kb|m|mb|g|gb)?$`)

// parseLogSize reads a size in KB, MB or GB, MB when no unit is given: "222", "20000 KB", "1.5 GB".
func parseLogSize(text string) (int, error) {
	match := logSizePattern.FindStringSubmatch(strings.ToLower(strings.TrimSpace(text)))
	if match == nil {
		return 0, errors.New("type a size, such as 222 MB, 20000 KB or 2 GB")
	}
	amount, err := strconv.ParseFloat(strings.Replace(match[1], ",", ".", 1), 64)
	if err != nil {
		return 0, errors.New("type a size, such as 222 MB, 20000 KB or 2 GB")
	}
	unitKB := map[string]float64{"k": 1, "kb": 1, "": 1 << 10, "m": 1 << 10, "mb": 1 << 10, "g": 1 << 20, "gb": 1 << 20}[match[2]]
	kb := math.Round(amount * unitKB)
	if kb < 1 || kb > maxLogSizeKB {
		return 0, fmt.Errorf("the size goes from 1 KB to %s", logstore.PrettySize(int64(maxLogSizeKB)<<10))
	}
	return int(kb), nil
}

// numberLines show a log limit's row and, while it is typed, how to save it and what is wrong.
func (a *App) numberLines(label string, row configRow, value string) []configLine {
	p := a.configPanel
	if p.number == nil || p.selected != row {
		return []configLine{{label: label, value: "◂ " + value + " ▸", row: row}}
	}
	lines := []configLine{{label: label, value: p.number.String(), row: row, typed: true}}
	if p.numberProblem != "" {
		lines = append(lines, configLine{label: "⚠ " + p.numberProblem, note: true, warning: true})
	}
	help := "enter saves · esc cancels"
	if row == logsSizeRow {
		help += " · KB, MB or GB"
	}
	return append(lines, configLine{label: help, note: true})
}
