# third_party

Copies of two libraries noct depends on, with fixes that make terminal output about three times
faster (2.5 → 7.6 MB/s through the emulator), that stop the renderer from leaving old cells
on screen, and that keep UTF-8 titles whole. They are packages of noct's own module (imports
rewritten, no `go.mod` of their own), not `replace` targets, so `go install …@latest` keeps working.
See ADR-011 in [docs/decisoes.md](../docs/decisoes.md).

| Library | Version | Change |
|---|---|---|
| `github.com/charmbracelet/ultraviolet` | v0.0.0-20260303162955-0b88c25f3fff | `Buffer.DeleteLineArea` moves whole lines instead of copying every cell when a terminal scrolls. Four fixes for cells left on screen or erased by mistake: `Line.Set` blanks a wide cell whose first half another wide cell covers; `Buffer.DeleteCellArea` moves whole cells, so deleting before a wide character keeps it; `transformLine` continues after the second half of a wide cell (`columnAfter`), which also removes an endless loop; `clearBottom` clears from the first blank line, not the one above it. `Buffer.InsertLineArea`, `DeleteLineArea`, `InsertCellArea` and `DeleteCellArea` keep to the part of the area inside the buffer, since a scroll margin can lie past the screen (a log replayed in a shorter pane); `third_party/vt/noct_margins_test.go` tests it. `TerminalScreen.Flush` writes the cursor move with the frame instead of leaving it for the next one. `noct_render_test.go` tests them. The `TerminalReader` decodes a two-byte escape key (alt+letter) as soon as more bytes follow it, instead of waiting while mouse motion keeps arriving; `noct_reader_test.go` tests it. `poll_default_test.go`'s `TestReader` cancels before the read starts, not while it may be starting: `Read` does not wait on the cancel signal, so the test failed now and then |
| `github.com/charmbracelet/x/vt` | v0.0.0-20260924144451-d676b019604b | `Scrollback.Push` drops the oldest line without shifting the whole history. An `init` makes the byte 0x9C part of OSC and DCS strings instead of ending them: in UTF-8 it is a continuation byte (`✳` is `E2 9C B3`), and ending the string there cut titles such as Claude Code's and printed the rest into the pane. `noct_string_test.go` tests it. `SendKey` sends Home and End as ESC O H and ESC O F in application cursor keys mode, like the arrows; `noct_key_test.go` tests it |

Each change is marked with a `noct:` comment. When updating a library, copy the new version over
its folder and apply the marked changes again, or drop them once upstream has an equivalent fix.

## License

Both libraries are © Charmbracelet, Inc. and MIT licensed. Each folder keeps the library's
original LICENSE file, which covers that folder including noct's changes; the rest of noct is
Apache 2.0. [NOTICE](../NOTICE) lists them for redistribution.
