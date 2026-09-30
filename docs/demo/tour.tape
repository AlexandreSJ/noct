# The second GIF of the README: search, the logs calendar, sessions, the config and the keys of
# every screen.
Output ../assets/tour.gif

Set Shell "bash"
Set FontSize 15
Set Width 1100
Set Height 620
Set Padding 16
Set Framerate 20
Set TypingSpeed 60ms
Set Theme { "background": "#1a1b26", "foreground": "#c0caf5", "cursor": "#c0caf5" }

Hide
Type "cd $NOCT_DEMO_WORK && clear"
Enter
Show

Type "noct"
Sleep 400ms
Enter
Sleep 1.5s

# The commands popup, then search: a script found by name runs with its form.
Ctrl+Space
Sleep 1.5s
Space
Sleep 800ms
Type "health"
Sleep 1s
Enter
Sleep 1.2s
Enter
Sleep 2s

# The logs calendar, each day colored by how many logs it has.
Ctrl+Space
Sleep 500ms
Type "v"
Sleep 3s
Tab
Sleep 800ms
PageDown
Sleep 1.2s
Tab
Sleep 500ms

# A second session, named in the tab bar.
Ctrl+Space
Sleep 500ms
Type "o"
Sleep 1.5s
Type "n"
Sleep 600ms
Type "work"
Sleep 500ms
Enter
Sleep 1.2s
Ctrl+Space
Sleep 500ms
Type "o"
Sleep 2s
Escape
Sleep 500ms

# The config: what a new tab opens, and a new prefix, warned about what it would take.
Ctrl+Space
Sleep 500ms
Type "c"
Sleep 1.5s
Enter
Sleep 800ms
Ctrl+C
Sleep 2.5s
Escape
Sleep 500ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 800ms
Right
Sleep 1s
Right
Sleep 1.2s

# The keys of every screen: choose which key of an action to change, then press the new one.
PageUp
Sleep 600ms
Down
Sleep 400ms
Enter
Sleep 1.5s
Enter
Sleep 1s
Right
Sleep 1s
Enter
Sleep 1s
Type "b"
Sleep 2.5s
Escape
Sleep 500ms
PageDown
Sleep 1s
PageDown
Sleep 1.5s
Escape
Sleep 500ms
Escape
Sleep 1s
