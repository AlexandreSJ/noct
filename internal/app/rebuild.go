package app

import (
	"os"
	"time"
)

// rebuiltMessage tells the user that attaching keeps the daemon's old code.
const rebuiltMessage = "noct was rebuilt: quit (prefix+q) and run noct again to use the new version"

// binaryStamp identifies the noct binary the daemon started from. The daemon keeps running its
// own code after a rebuild, so attaching with a new binary would otherwise look like changes
// that did nothing.
type binaryStamp struct {
	path    string
	modTime time.Time
}

func currentBinary() binaryStamp {
	path, err := os.Executable()
	if err != nil {
		return binaryStamp{}
	}
	return stampOf(path)
}

func stampOf(path string) binaryStamp {
	info, err := os.Stat(path)
	if err != nil {
		return binaryStamp{}
	}
	return binaryStamp{path: path, modTime: info.ModTime()}
}

// rebuilt tells whether the binary on disk changed since the stamp was taken.
func (s binaryStamp) rebuilt() bool {
	if s.path == "" {
		return false
	}
	now := stampOf(s.path)
	return now.path != "" && !now.modTime.Equal(s.modTime)
}
