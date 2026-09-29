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
}

func (s *Splitter) Split(data []byte) []Segment {
	buf := append(s.pending, data...)
	s.pending = nil
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
		end := bytes.Index(buf[bodyStart:], []byte(commandEnd))
		if end < 0 {
			s.keepUnfinished(buf[start:])
			break
		}
		cmd := ParseCommand(buf[bodyStart : bodyStart+end])
		segments = append(segments, Segment{Command: &cmd})
		buf = buf[bodyStart+end+len(commandEnd):]
	}
	return segments
}

func (s *Splitter) keepUnfinished(command []byte) {
	if len(command) <= maxPendingCommand {
		s.pending = append([]byte(nil), command...)
	}
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
