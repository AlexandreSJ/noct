// Package schedule decides when flows start by themselves: on their cron schedule, when the
// files they watch change, or after another flow ends. It only decides; whoever owns it runs them.
package schedule

import (
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// maxWatchedFiles bounds a watch's walk, so a pattern over a huge folder cannot stall ticks.
const maxWatchedFiles = 50000

// Scheduler looks at the enabled flows' triggers on each tick. It keeps what it saw last: the
// minute, each watch's files and each flow's last end.
type Scheduler struct {
	// Dir is where relative watch paths start.
	Dir string
	// ResultsDir holds each flow's last result; a new end there is what after triggers wait for.
	ResultsDir string
	// Start runs a flow, saying what started it.
	Start func(name, reason string)

	// since is the first tick: runs that ended before it are old news.
	since   time.Time
	minute  time.Time
	watches map[string]*watchState
	ends    map[string]resultSeen
}

type watchState struct {
	signature uint64
	// due is when the files, changed and then left alone, start the flow; zero when not waiting.
	due time.Time
}

// resultSeen is a flow's last result file as seen on a tick: read again only when it changes.
type resultSeen struct {
	modified time.Time
	end      time.Time
}

// Tick starts the enabled flows whose triggers say so at now.
func (s *Scheduler) Tick(now time.Time, flows []flow.Flow, enabled map[string]bool) {
	if s.watches == nil {
		s.watches, s.ends, s.since = map[string]*watchState{}, map[string]resultSeen{}, now
	}
	s.checkCron(now, flows, enabled)
	s.checkEnds(flows, enabled)
	s.checkWatches(now, flows, enabled)
}

// checkCron runs, once in each new minute, the flows scheduled for it. The first tick only
// notes its minute: a restart within a minute must not run a flow twice.
func (s *Scheduler) checkCron(now time.Time, flows []flow.Flow, enabled map[string]bool) {
	minute := now.Truncate(time.Minute)
	if !minute.After(s.minute) {
		return
	}
	first := s.minute.IsZero()
	s.minute = minute
	if first {
		return
	}
	for _, f := range flows {
		for _, t := range f.Triggers {
			if t.Cron == "" || !enabled[f.Name] {
				continue
			}
			if cron, err := flow.ParseCron(t.Cron); err == nil && cron.Matches(minute) {
				s.Start(f.Name, t.Label())
			}
		}
	}
}

// checkEnds starts the flows waiting for another flow that ended since the last tick. Any run
// counts, whoever started it, since every run leaves its result in ResultsDir.
func (s *Scheduler) checkEnds(flows []flow.Flow, enabled map[string]bool) {
	for _, f := range flows {
		r, ended := s.newEnd(f.Name)
		if !ended {
			continue
		}
		for _, waiting := range flows {
			for _, t := range waiting.Triggers {
				if t.After != nil && enabled[waiting.Name] && t.After.Waits(f.Name, r.OK) {
					s.Start(waiting.Name, "after "+f.Name)
				}
			}
		}
	}
}

// newEnd reads a flow's last result when its file changed, and tells whether it ended a run
// after the one seen before, or, the first time, after the scheduler started.
func (s *Scheduler) newEnd(name string) (flow.Result, bool) {
	info, err := os.Stat(filepath.Join(s.ResultsDir, name+".json"))
	if err != nil {
		return flow.Result{}, false
	}
	seen, known := s.ends[name]
	if known && info.ModTime().Equal(seen.modified) {
		return flow.Result{}, false
	}
	r, ok := flow.LastResult(s.ResultsDir, name)
	if !ok {
		return flow.Result{}, false
	}
	s.ends[name] = resultSeen{modified: info.ModTime(), end: r.End}
	since := s.since
	if known {
		since = seen.end
	}
	return r, r.End.After(since)
}

// checkWatches starts a flow once the files it watches changed and stayed unchanged for its
// debounce.
func (s *Scheduler) checkWatches(now time.Time, flows []flow.Flow, enabled map[string]bool) {
	for _, f := range flows {
		for i, t := range f.Triggers {
			if t.Watch == nil || !enabled[f.Name] {
				continue
			}
			key := fmt.Sprintf("%s#%d", f.Name, i)
			signature := s.signature(t.Watch.Paths)
			state, known := s.watches[key]
			if !known {
				s.watches[key] = &watchState{signature: signature}
				continue
			}
			if signature != state.signature {
				state.signature, state.due = signature, now.Add(t.Watch.DebounceTime())
				continue
			}
			if !state.due.IsZero() && !now.Before(state.due) {
				state.due = time.Time{}
				s.Start(f.Name, t.Label())
			}
		}
	}
}

// signature sums up the files the patterns match: their paths, sizes and modification times.
func (s *Scheduler) signature(patterns []string) uint64 {
	h := fnv.New64a()
	for _, pattern := range patterns {
		pattern = s.absolute(pattern)
		root := staticRoot(pattern)
		files := 0
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || files >= maxWatchedFiles {
				return filepath.SkipDir // unreadable folders are skipped, as is anything past the bound
			}
			if d.IsDir() {
				if d.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if !flow.MatchPath(filepath.ToSlash(pattern), filepath.ToSlash(path)) {
				return nil
			}
			files++
			if info, err := d.Info(); err == nil {
				fmt.Fprintf(h, "%s\x00%d\x00%d\x00", path, info.Size(), info.ModTime().UnixNano())
			}
			return nil
		})
	}
	return h.Sum64()
}

// absolute makes a watch path absolute: ~ is the home folder, a relative path starts at Dir.
func (s *Scheduler) absolute(pattern string) string {
	if rest, ok := strings.CutPrefix(pattern, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	if filepath.IsAbs(pattern) {
		return filepath.Clean(pattern)
	}
	return filepath.Join(s.Dir, pattern)
}

// staticRoot is the folder a pattern's walk starts at: its part before the first wildcard.
func staticRoot(pattern string) string {
	parts := strings.Split(pattern, string(filepath.Separator))
	for i, part := range parts {
		if strings.ContainsAny(part, "*?[") {
			return strings.Join(parts[:i], string(filepath.Separator))
		}
	}
	return filepath.Dir(pattern)
}
