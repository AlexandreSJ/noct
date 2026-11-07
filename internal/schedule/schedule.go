// Package schedule decides when flows start by themselves: on their cron schedule, when the
// files they watch change, or after another flow ends. It only decides; whoever owns it runs them.
package schedule

import (
	"fmt"
	"hash/fnv"
	"io"
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
	since time.Time
	// minute is the last minute cron looked at, as the wall clock showed it (see wallClock).
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

// checkCron runs, once in each new minute of the wall clock, the flows scheduled for it. The
// first tick only notes its minute: a restart within a minute must not run a flow twice. Minutes
// are compared as the clock shows them, so when clocks go back an hour its minutes, which come
// again, do not run anything twice.
func (s *Scheduler) checkCron(now time.Time, flows []flow.Flow, enabled map[string]bool) {
	minute := wallClock(now)
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
			if cron, err := flow.ParseCron(t.Cron); err == nil && cron.Matches(now) {
				s.Start(f.Name, t.Label())
			}
		}
	}
}

// wallClock is what t's clock shows, as the same reading in UTC, so two readings compare by
// what the clock showed rather than by when they happened.
func wallClock(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC)
}

// checkEnds starts the flows waiting for another flow that ended since the last tick. Any run
// counts, whoever started it, since every run leaves its result in ResultsDir.
func (s *Scheduler) checkEnds(flows []flow.Flow, enabled map[string]bool) {
	for _, f := range flows {
		r, ended := s.newEnd(f.Name)
		if ended {
			s.startWaitingFor(f.Name, r.OK, flows, enabled)
		}
	}
}

// startWaitingFor starts the enabled flows with an after trigger for the flow that just ended.
func (s *Scheduler) startWaitingFor(ended string, ok bool, flows []flow.Flow, enabled map[string]bool) {
	for _, waiting := range flows {
		for _, t := range waiting.Triggers {
			if t.After != nil && enabled[waiting.Name] && t.After.Waits(ended, ok) {
				s.Start(waiting.Name, "after "+ended)
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

func (s *Scheduler) checkWatches(now time.Time, flows []flow.Flow, enabled map[string]bool) {
	for _, f := range flows {
		for i, t := range f.Triggers {
			if t.Watch != nil && enabled[f.Name] {
				s.checkWatch(now, f.Name, i, t)
			}
		}
	}
}

// checkWatch starts the flow once the files of its index-th trigger changed and then stayed
// unchanged for the trigger's debounce.
func (s *Scheduler) checkWatch(now time.Time, name string, index int, t flow.Trigger) {
	key := fmt.Sprintf("%s#%d", name, index)
	signature := s.signature(t.Watch.Paths)
	state, known := s.watches[key]
	if !known {
		s.watches[key] = &watchState{signature: signature}
		return
	}
	if signature != state.signature {
		state.signature, state.due = signature, now.Add(t.Watch.DebounceTime())
		return
	}
	if !state.due.IsZero() && !now.Before(state.due) {
		state.due = time.Time{}
		s.Start(name, t.Label())
	}
}

// signature sums up the files the patterns match: their paths, sizes and modification times.
func (s *Scheduler) signature(patterns []string) uint64 {
	h := fnv.New64a()
	for _, pattern := range patterns {
		hashMatchingFiles(h, s.absolute(pattern))
	}
	return h.Sum64()
}

// hashMatchingFiles writes into h the path, size and modification time of each file the
// absolute pattern matches.
func hashMatchingFiles(h io.Writer, pattern string) {
	if !strings.ContainsAny(pattern, "*?[") {
		if info, err := os.Stat(pattern); err == nil && !info.IsDir() {
			fmt.Fprintf(h, "%s\x00%d\x00%d\x00", pattern, info.Size(), info.ModTime().UnixNano())
		}
		return // one file needs no walk through its folder
	}
	files := 0
	_ = filepath.WalkDir(staticRoot(pattern), func(path string, d fs.DirEntry, err error) error {
		if err != nil || files >= maxWatchedFiles {
			return filepath.SkipDir // unreadable folders are skipped, as is anything past the bound
		}
		if d.IsDir() {
			if !flow.MayMatchBelow(filepath.ToSlash(pattern), filepath.ToSlash(path)) {
				return filepath.SkipDir
			}
			return skipGitDir(d)
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

func skipGitDir(d fs.DirEntry) error {
	if d.Name() == ".git" {
		return filepath.SkipDir
	}
	return nil
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
