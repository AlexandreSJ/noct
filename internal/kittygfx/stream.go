package kittygfx

import "bytes"

// maxPendingCommand bounds an unfinished command kept between reads; a larger one is dropped.
const maxPendingCommand = 32 << 20

// Segment is a piece of program output: either plain text for the emulator or a graphics command.
type Segment struct {
	Text    []byte
	Command *Command
}

// Splitter separates graphics commands from the rest of a program's output, in order.
// Commands can span reads, so an unfinished one waits for the rest.
type Splitter struct {
	pending []byte
	// searched is how much of an unfinished command in pending was searched for its end, so a
	// command sent over many reads is searched once, not again from its start on every read.
	searched int
}

func (s *Splitter) Split(data []byte) []Segment {
	buf := append(s.pending, data...)
	searched := s.searched
	s.pending, s.searched = nil, 0
	var segments []Segment
	for len(buf) > 0 {
		start := bytes.Index(buf, []byte(commandStart))
		if start < 0 {
			text, rest := splitTrailingPrefix(buf)
			segments = appendText(segments, text)
			s.pending = rest
			break
		}
		segments = appendText(segments, buf[:start])
		bodyStart := start + len(commandStart)
		// An end marker cut between reads starts in the last byte searched.
		searchFrom := max(bodyStart, searched-len(commandEnd)+1)
		end := bytes.Index(buf[searchFrom:], []byte(commandEnd))
		if end < 0 {
			s.keepUnfinished(buf[start:], searched > 0)
			break
		}
		searched = 0 // only a command at the start of buf was searched before
		end += searchFrom
		cmd := ParseCommand(buf[bodyStart:end])
		segments = append(segments, Segment{Command: &cmd})
		buf = buf[end+len(commandEnd):]
	}
	return segments
}

// keepUnfinished keeps a command until its end arrives. A new one is copied, so it does not hold
// the read it came in; one kept before already owns its buffer, which grows in place, so a long
// upload is copied a bounded number of times instead of once per read.
func (s *Splitter) keepUnfinished(command []byte, keptBefore bool) {
	if len(command) > maxPendingCommand {
		return
	}
	if !keptBefore {
		command = append([]byte(nil), command...)
	}
	s.pending, s.searched = command, len(command)
}

// splitTrailingPrefix holds back an end of buf that could be the start of a command,
// such as a lone ESC, until the next read tells what it is.
func splitTrailingPrefix(buf []byte) (text, rest []byte) {
	for size := min(len(commandStart)-1, len(buf)); size > 0; size-- {
		if bytes.HasSuffix(buf, []byte(commandStart[:size])) {
			cut := len(buf) - size
			return buf[:cut], append([]byte(nil), buf[cut:]...)
		}
	}
	return buf, nil
}

func appendText(segments []Segment, text []byte) []Segment {
	if len(text) == 0 {
		return segments
	}
	return append(segments, Segment{Text: append([]byte(nil), text...)})
}
