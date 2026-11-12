package app

import (
	"os"

	"github.com/AlexandreSJ/noct/internal/atomicfile"
	"github.com/AlexandreSJ/noct/internal/flow"
)

// Every change the inspect makes to a flow can be taken back: z undoes the last, y does it again.
// The history is each flow's file as it was, kept while noct runs.

// maxFlowHistory bounds how many changes of a flow can be undone.
const maxFlowHistory = 100

type flowHistory struct {
	undo, redo [][]byte
}

// rememberFlow keeps the flow's file as it is, before a change, and forgets what was undone.
func (a *App) rememberFlow(f flow.Flow) {
	content, err := os.ReadFile(f.File)
	if err != nil {
		return
	}
	if a.flowHistories == nil {
		a.flowHistories = map[string]*flowHistory{}
	}
	h := a.flowHistories[f.Name]
	if h == nil {
		h = &flowHistory{}
		a.flowHistories[f.Name] = h
	}
	h.undo = append(h.undo, content)
	if len(h.undo) > maxFlowHistory {
		h.undo = h.undo[1:]
	}
	h.redo = nil
}

// undoFlow puts the flow back as it was before its last change (redo false), or does again the
// last change undone (redo true).
func (a *App) undoFlow(f flow.Flow, redo bool) {
	what := map[bool]string{false: "undo", true: "redo"}[redo]
	h := a.flowHistories[f.Name]
	if h == nil {
		h = &flowHistory{}
	}
	from, to := &h.undo, &h.redo
	if redo {
		from, to = &h.redo, &h.undo
	}
	if len(*from) == 0 {
		a.notice = "nothing to " + what + " in flow " + f.Name
		return
	}
	current, err := os.ReadFile(f.File)
	if err != nil {
		a.statusMessage = err.Error()
		return
	}
	last := (*from)[len(*from)-1]
	if err := atomicfile.Write(f.File, last, 0o644); err != nil {
		a.statusMessage = err.Error()
		return
	}
	*from, *to = (*from)[:len(*from)-1], append(*to, current)
	a.reloadFlows()
	a.notice = what + " in flow " + f.Name
}
