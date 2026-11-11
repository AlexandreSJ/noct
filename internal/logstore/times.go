package logstore

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// Beside each log file, its times file keeps when its lines started: a "line unixseconds" record
// each time a line starts in a second later than the last record's, so a burst of output costs one.

// TimeLayout is how a line's start shows.
const TimeLayout = "2006-01-02 15-04-05"

const timesSuffix = ".times"

// TimesPath is the times file of a log file (the log or its rotated part).
func TimesPath(file string) string {
	return file + timesSuffix
}

// LineTime says that the lines from Line on, until the next record, started at At.
type LineTime struct {
	Line int
	At   time.Time
}

// LineTimes are the records of the log file, in order; none without its times file.
func LineTimes(file string) []LineTime {
	content, _ := os.ReadFile(TimesPath(file))
	var times []LineTime
	for _, record := range strings.Split(string(content), "\n") {
		line, at, ok := strings.Cut(record, " ")
		n, err1 := strconv.Atoi(line)
		seconds, err2 := strconv.ParseInt(at, 10, 64)
		if ok && err1 == nil && err2 == nil {
			times = append(times, LineTime{n, time.Unix(seconds, 0)})
		}
	}
	return times
}

// lineClock counts a file's lines as they are written and records when they start.
type lineClock struct {
	lines   int // lines started so far
	atStart bool
	last    int64
}

func (c *lineClock) reset(lines int) {
	*c = lineClock{lines: lines, atStart: true}
}

// written notes p, written at now to the log file, recording the first line it starts when now
// is a second later than the last record.
func (c *lineClock) written(file string, p []byte, now time.Time) {
	first := 0
	for len(p) > 0 {
		if c.atStart {
			c.lines++
			c.atStart = false
			if first == 0 {
				first = c.lines
			}
		}
		newline := bytes.IndexByte(p, '\n')
		if newline < 0 {
			break
		}
		c.atStart = true
		p = p[newline+1:]
	}
	if first == 0 || now.Unix() <= c.last {
		return
	}
	c.last = now.Unix()
	times, err := os.OpenFile(TimesPath(file), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return // the lines only show without their time
	}
	_, _ = fmt.Fprintf(times, "%d %d\n", first, c.last)
	_ = times.Close()
}

// LineRange are the lines PrintLines prints, counted over its files from 1: From (1 when 0) to To
// (the end when 0); Follow goes on printing what the last file gets, until stop closes.
type LineRange struct {
	From, To int
	Follow   bool
}

// pollInterval is how often a followed file is read again for new output.
const pollInterval = 200 * time.Millisecond

// PrintLines writes the lines of files, in order, each after the time it started, dim, to out.
// A line with no time known gets blanks in its place.
func PrintLines(out io.Writer, r LineRange, stop <-chan struct{}, files ...string) error {
	w := bufio.NewWriter(out)
	defer w.Flush()
	line := 0
	for i, name := range files {
		file, err := os.Open(name)
		if err != nil {
			return err
		}
		p := &timedPrinter{out: w, times: LineTimes(name), at: -1, line: line, r: r}
		follow := r.Follow && i == len(files)-1
		err = p.copy(file, follow, stop)
		file.Close()
		if err != nil {
			return err
		}
		line = p.line
		if r.To > 0 && line >= r.To {
			return nil
		}
	}
	return nil
}

type timedPrinter struct {
	out     *bufio.Writer
	times   []LineTime
	at      int // the record of the last line printed, or -1
	line    int // over all the files
	inFile  int // lines started in this file
	midLine bool
	r       LineRange
}

func (p *timedPrinter) copy(file *os.File, follow bool, stop <-chan struct{}) error {
	reader := bufio.NewReader(file)
	for {
		chunk, err := reader.ReadSlice('\n')
		if len(chunk) > 0 {
			p.print(chunk)
		}
		switch {
		case err == nil, err == bufio.ErrBufferFull:
			continue
		case err != io.EOF:
			return err
		case !follow:
			return nil
		}
		if err := p.out.Flush(); err != nil {
			return err
		}
		select {
		case <-stop:
			return nil
		case <-time.After(pollInterval):
		}
	}
}

func (p *timedPrinter) print(chunk []byte) {
	if !p.midLine {
		p.line++
		p.inFile++
	}
	shown := p.line >= max(p.r.From, 1) && (p.r.To <= 0 || p.line <= p.r.To)
	if shown && !p.midLine {
		p.out.WriteString(p.timeOf(p.inFile))
		p.out.WriteByte(' ')
	}
	if shown {
		p.out.Write(chunk)
	}
	p.midLine = chunk[len(chunk)-1] != '\n'
}

// timeOf is when the file's line started, shown, or blanks when no record covers it. Lines come
// in order, so the record only moves on.
func (p *timedPrinter) timeOf(line int) string {
	for p.at+1 < len(p.times) && p.times[p.at+1].Line <= line {
		p.at++
	}
	if p.at < 0 {
		return strings.Repeat(" ", len(TimeLayout))
	}
	return dim(p.times[p.at].At.Format(TimeLayout))
}

func dim(text string) string {
	return "\x1b[2m" + text + "\x1b[22m"
}
