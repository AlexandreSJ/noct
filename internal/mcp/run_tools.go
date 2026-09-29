package mcp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/logstore"
	"github.com/AlexandreSJ/noct/internal/script"
)

// Bounds of what the run tools send back, so one answer never floods the AI's context.
const (
	logTailLines   = 50
	maxStepOutput  = 4 << 10
	maxWaitSeconds = 600
	maxReadLines   = 2000
	maxMatches     = 500
	defaultLimit   = 50
)

func runTools(d Deps) []Tool {
	return []Tool{
		{
			Name: "list_runs", Title: "List runs", ReadOnly: true, Policy: config.PolicyAllow,
			Description: "Lists runs of scripts and flows, newest first, as the calendar does: run_id, name, when, status (ok, failed, running), exit code, and the trigger that started it.",
			Params: []Param{
				{Name: "name", Type: "string", Description: "Only runs of this script or flow."},
				{Name: "status", Type: "string", Description: "Only runs that ended so.", Enum: []string{"ok", "failed", "running"}},
				{Name: "since", Type: "string", Description: "Only runs from this day on: YYYY-MM-DD."},
				{Name: "until", Type: "string", Description: "Only runs up to this day: YYYY-MM-DD."},
				{Name: "limit", Type: "integer", Description: "At most this many runs; 50 by default."},
			},
			Run: d.listRuns,
		},
		{
			Name: "get_run", Title: "Read a run's result", ReadOnly: true, Policy: config.PolicyAllow,
			Description: "The structured result of a run: status, exit code, duration and, for a flow, each step in order with its path, id, status, exit code, captures, output (up to 4 KB) and why it failed; and the last lines of its log. Give run_id, or flow for its last run. wait_seconds waits for a running run to end.",
			Params: []Param{
				{Name: "run_id", Type: "string", Description: "What run and list_runs give."},
				{Name: "flow", Type: "string", Description: "A flow's name: its last run."},
				{Name: "wait_seconds", Type: "integer", Description: "Wait up to this long for the run to end; 0 answers at once. At most 600."},
			},
			Run: d.getRun,
		},
		{
			Name: "search_logs", Title: "Search the logs", ReadOnly: true, Policy: config.PolicyAllow,
			Description: "Searches what runs and terminals printed, by regex (Go syntax), line by line, without colors. Narrow it with run_id, name or days; each match comes with lines of context.",
			Params: []Param{
				{Name: "pattern", Type: "string", Description: "A regex; (?i) for any case.", Required: true},
				{Name: "run_id", Type: "string", Description: "Only this run's log."},
				{Name: "name", Type: "string", Description: "Only logs whose name has this text."},
				{Name: "since", Type: "string", Description: "From this day: YYYY-MM-DD."},
				{Name: "until", Type: "string", Description: "Up to this day: YYYY-MM-DD."},
				{Name: "context", Type: "integer", Description: "Lines before and after each match; 2 by default."},
				{Name: "max_matches", Type: "integer", Description: "At most this many matches; 50 by default, 500 at most."},
			},
			Run: d.searchLogs,
		},
		{
			Name: "read_log", Title: "Read a log", ReadOnly: true, Policy: config.PolicyAllow,
			Description: "Reads a run's log (the run_id is its path) a page of lines at a time, without colors.",
			Params: []Param{
				{Name: "run_id", Type: "string", Description: "The run, or any log path list_runs or search_logs gave.", Required: true},
				{Name: "offset", Type: "integer", Description: "The first line, from 0; a negative offset counts from the end."},
				{Name: "limit", Type: "integer", Description: "How many lines; 200 by default, 2000 at most."},
			},
			Run: d.readLog,
		},
	}
}

func runsPath() string {
	return filepath.Join(logstore.Dir(), script.RunsFile)
}

type runInfo struct {
	RunID   string    `json:"run_id,omitempty"`
	Name    string    `json:"name"`
	Kind    string    `json:"kind"`
	At      time.Time `json:"started_at"`
	Status  string    `json:"status"`
	Exit    *int      `json:"exit,omitempty"`
	Trigger string    `json:"trigger,omitempty"`
}

func describeRun(r script.Run) runInfo {
	info := runInfo{RunID: r.Log, Name: r.Script, Kind: "script", At: r.At, Exit: r.Exit, Trigger: r.Trigger, Status: runStatus(r)}
	if r.Flow != "" {
		info.Name, info.Kind = r.Flow, "flow"
	}
	return info
}

// runStatus is how a run ended; a run with no exit code is running, or was cut short.
func runStatus(r script.Run) string {
	switch {
	case r.Exit == nil:
		return "running"
	case *r.Exit == 0:
		return "ok"
	}
	return "failed"
}

// dayRange reads since and until, whole days, either one left open.
func dayRange(since, until string) (time.Time, time.Time, error) {
	from, to := time.Time{}, time.Now().AddDate(100, 0, 0)
	var err error
	if since != "" {
		if from, err = time.ParseInLocation(logstore.DayLayout, since, time.Local); err != nil {
			return from, to, fmt.Errorf("since %q is not YYYY-MM-DD", since)
		}
	}
	if until != "" {
		if to, err = time.ParseInLocation(logstore.DayLayout, until, time.Local); err != nil {
			return from, to, fmt.Errorf("until %q is not YYYY-MM-DD", until)
		}
		to = to.AddDate(0, 0, 1)
	}
	return from, to, nil
}

func (d Deps) listRuns(args json.RawMessage) (any, error) {
	var ask struct {
		Name, Status, Since, Until string
		Limit                      int
	}
	if err := decode(args, &ask); err != nil {
		return nil, err
	}
	from, to, err := dayRange(ask.Since, ask.Until)
	if err != nil {
		return nil, err
	}
	runs, err := script.Runs(runsPath())
	if err != nil {
		return nil, err
	}
	limit := ask.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	found := []runInfo{}
	for _, r := range slices.Backward(runs) {
		info := describeRun(r)
		if (ask.Name != "" && info.Name != ask.Name) || (ask.Status != "" && info.Status != ask.Status) || r.At.Before(from) || !r.At.Before(to) {
			continue
		}
		found = append(found, info)
		if len(found) == limit {
			break
		}
	}
	return map[string]any{"runs": found}, nil
}

// findRun finds a run by its id, or a flow's newest run.
func findRun(runID, flowName string) (script.Run, error) {
	runs, err := script.Runs(runsPath())
	if err != nil {
		return script.Run{}, err
	}
	for _, r := range slices.Backward(runs) {
		if (runID != "" && r.Log == runID) || (runID == "" && flowName != "" && r.Flow == flowName) {
			return r, nil
		}
	}
	if runID != "" {
		return script.Run{}, fmt.Errorf("there is no run %s; list_runs tells them", runID)
	}
	return script.Run{}, fmt.Errorf("flow %s has no run on record", flowName)
}

type stepInfo struct {
	Path       string            `json:"path"`
	ID         string            `json:"id,omitempty"`
	Kind       string            `json:"kind,omitempty"`
	Status     string            `json:"status"`
	Exit       int               `json:"exit"`
	DurationMS int64             `json:"duration_ms"`
	Captures   map[string]string `json:"captures,omitempty"`
	Output     string            `json:"output,omitempty"`
	Reason     string            `json:"reason,omitempty"`
	Tried      bool              `json:"tried_alone,omitempty"`
}

func (d Deps) getRun(args json.RawMessage) (any, error) {
	var ask struct {
		RunID       string `json:"run_id"`
		Flow        string `json:"flow"`
		WaitSeconds int    `json:"wait_seconds"`
	}
	if err := decode(args, &ask); err != nil {
		return nil, err
	}
	if ask.RunID == "" && ask.Flow == "" {
		return nil, errors.New("give run_id, or flow for its last run")
	}
	run, err := findRun(ask.RunID, ask.Flow)
	deadline := d.Now().Add(time.Duration(min(ask.WaitSeconds, maxWaitSeconds)) * time.Second)
	for err == nil && run.Exit == nil && d.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		run, err = findRun(run.Log, "")
	}
	if err != nil {
		return nil, err
	}
	answer := map[string]any{"run": describeRun(run), "log_ref": run.Log, "log_tail": logTail(run.Log, logTailLines)}
	if run.Flow != "" {
		d.addFlowSteps(answer, run)
	}
	return answer, nil
}

// addFlowSteps puts in a flow run's answer each step's result, in the order the flow runs them.
func (d Deps) addFlowSteps(answer map[string]any, run script.Run) {
	result, ok := flow.ReadResult(logstore.ResultPath(run.Log))
	if !ok {
		answer["steps_note"] = "this run kept no result of its steps: it ran before noct kept them, or has not ended"
		return
	}
	f, _ := flow.Find(d.flows(), run.Flow) // a flow deleted since still has its results
	paths := make([]string, 0, len(result.Steps))
	for path := range result.Steps {
		paths = append(paths, path)
	}
	slices.SortFunc(paths, comparePaths)
	steps := []stepInfo{}
	for _, path := range paths {
		r := result.Steps[path]
		info := stepInfo{Path: path, Status: string(r.Status), Exit: r.Exit, DurationMS: r.Duration.Milliseconds(),
			Captures: r.Captures, Output: tail(r.Output, maxStepOutput), Reason: r.Message, Tried: r.Tried}
		if s, found := f.Step(path); found {
			info.ID, info.Kind = s.ID, string(s.Kind())
		}
		steps = append(steps, info)
	}
	answer["steps"] = steps
	answer["ok"] = result.OK
	answer["duration_ms"] = result.End.Sub(result.Start).Milliseconds()
}

// comparePaths orders step paths as the flow runs them: 0 before 0/steps/1 before 1, and 2 before 10.
func comparePaths(a, b string) int {
	as, bs := strings.Split(a, "/"), strings.Split(b, "/")
	for i := 0; i < min(len(as), len(bs)); i++ {
		x, errX := strconv.Atoi(as[i])
		y, errY := strconv.Atoi(bs[i])
		switch {
		case errX == nil && errY == nil && x != y:
			return x - y
		case (errX != nil || errY != nil) && as[i] != bs[i]:
			return strings.Compare(as[i], bs[i])
		}
	}
	return len(as) - len(bs)
}

// tail is the end of a text, at most limit bytes of it, from the start of a line when it can.
func tail(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	cut := text[len(text)-limit:]
	if i := strings.IndexByte(cut, '\n'); i >= 0 && i < len(cut)-1 {
		cut = cut[i+1:]
	}
	return "…\n" + cut
}

// logLines reads a log's lines, its rotated older part first, without colors.
func logLines(path string) ([]string, error) {
	if !strings.HasPrefix(path, logstore.LogsDir()) {
		return nil, fmt.Errorf("%s is not one of noct's logs", path)
	}
	var lines []string
	for _, file := range []string{path + ".1", path} {
		f, err := os.Open(file)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64<<10), 4<<20)
		for scanner.Scan() {
			lines = append(lines, strings.TrimRight(ansi.Strip(scanner.Text()), "\r"))
		}
		f.Close()
	}
	return lines, nil
}

func logTail(path string, n int) string {
	lines, err := logLines(path)
	if err != nil || len(lines) == 0 {
		return ""
	}
	return strings.Join(lines[max(len(lines)-n, 0):], "\n")
}

func (d Deps) readLog(args json.RawMessage) (any, error) {
	var ask struct {
		RunID  string `json:"run_id"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := decode(args, &ask); err != nil {
		return nil, err
	}
	lines, err := logLines(ask.RunID)
	if err != nil {
		return nil, err
	}
	limit := ask.Limit
	if limit <= 0 {
		limit = 200
	}
	limit = min(limit, maxReadLines)
	start := ask.Offset
	if start < 0 {
		start = max(len(lines)+start, 0)
	}
	start = min(start, len(lines))
	end := min(start+limit, len(lines))
	answer := map[string]any{"run_id": ask.RunID, "offset": start, "lines": lines[start:end], "total_lines": len(lines)}
	if end < len(lines) {
		answer["next_offset"] = end
	}
	return answer, nil
}

type match struct {
	RunID  string   `json:"run_id"`
	Name   string   `json:"name"`
	At     string   `json:"at"`
	Line   int      `json:"line"`
	Text   string   `json:"text"`
	Before []string `json:"before,omitempty"`
	After  []string `json:"after,omitempty"`
}

func (d Deps) searchLogs(args json.RawMessage) (any, error) {
	var ask struct {
		Pattern    string `json:"pattern"`
		RunID      string `json:"run_id"`
		Name       string `json:"name"`
		Since      string `json:"since"`
		Until      string `json:"until"`
		Context    *int   `json:"context"`
		MaxMatches int    `json:"max_matches"`
	}
	if err := decode(args, &ask); err != nil {
		return nil, err
	}
	re, err := regexp.Compile(ask.Pattern)
	if err != nil {
		return nil, fmt.Errorf("pattern: %w", err)
	}
	from, to, err := dayRange(ask.Since, ask.Until)
	if err != nil {
		return nil, err
	}
	context := 2
	if ask.Context != nil {
		context = min(max(*ask.Context, 0), 20)
	}
	limit := ask.MaxMatches
	if limit <= 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxMatches)
	entries, err := logstore.List(logstore.LogsDir())
	if err != nil {
		return nil, err
	}
	matches, total := []match{}, 0
	for _, e := range slices.Backward(entries) {
		if (ask.RunID != "" && e.Path != ask.RunID) || (ask.Name != "" && !strings.Contains(e.Name, ask.Name)) || e.At.Before(from) || !e.At.Before(to) {
			continue
		}
		lines, err := logLines(e.Path)
		if err != nil {
			continue // a log removed meanwhile has nothing to find
		}
		for i, line := range lines {
			if !re.MatchString(line) {
				continue
			}
			total++
			if len(matches) < limit {
				matches = append(matches, match{RunID: e.Path, Name: e.Name, At: e.At.Format(time.RFC3339), Line: i,
					Text: line, Before: lines[max(i-context, 0):i], After: lines[i+1 : min(i+1+context, len(lines))]})
			}
		}
	}
	return map[string]any{"matches": matches, "total_matches": total, "shown": len(matches)}, nil
}
