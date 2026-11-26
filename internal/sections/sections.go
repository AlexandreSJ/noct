// Package sections marks parts of a program's output that only one of two views shows, as a
// flow run's headers only in its full view and its exit line only in its clean one. A mark is an
// OSC sequence terminals ignore; a part without one shows in both.
package sections

import "bytes"

const (
	prefix = "\x1b]7770;"
	// Full starts what only the full view shows, Clean what only the clean one shows, and Both
	// goes back to what both show.
	Full  = prefix + "full\x07"
	Clean = prefix + "clean\x07"
	Both  = prefix + "both\x07"
)

type View int

const (
	CleanView View = iota
	FullView
)

// Filter gives what a view shows of output written to it a piece at a time; a mark cut between
// two pieces waits for the rest.
type Filter struct {
	View    View
	section string
	pending []byte
}

func (f *Filter) Write(data []byte) []byte {
	buf := append(f.pending, data...)
	f.pending = nil
	var out []byte
	for len(buf) > 0 {
		at := bytes.Index(buf, []byte(prefix))
		if at < 0 {
			keep := partialPrefix(buf)
			out = f.emit(out, buf[:len(buf)-keep])
			f.pending = append(f.pending, buf[len(buf)-keep:]...)
			break
		}
		out = f.emit(out, buf[:at])
		end := bytes.IndexByte(buf[at:], '\a')
		if end < 0 {
			f.pending = append(f.pending, buf[at:]...)
			break
		}
		f.section = string(buf[at : at+end+1])
		buf = buf[at+end+1:]
	}
	return out
}

func (f *Filter) emit(out, text []byte) []byte {
	if f.section == Full && f.View != FullView || f.section == Clean && f.View != CleanView {
		return out
	}
	return append(out, text...)
}

// partialPrefix is how many bytes at the end of buf could start a mark.
func partialPrefix(buf []byte) int {
	for n := min(len(prefix)-1, len(buf)); n > 0; n-- {
		if bytes.HasSuffix(buf, []byte(prefix[:n])) {
			return n
		}
	}
	return 0
}
