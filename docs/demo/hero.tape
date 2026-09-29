# The first GIF of the README: home, a terminal, a script and a flow running.
Output ../assets/hero.gif

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
Sleep 2.5s

# A terminal tab.
Ctrl+Space
Sleep 700ms
Type "t"
Sleep 1s
Type "noct ls"
Sleep 300ms
Enter
Sleep 1.5s

# The scripts panel beside it: open the api folder and run health.
Ctrl+Space
Sleep 700ms
Type "s"
Sleep 1.2s
Enter
Sleep 700ms
Down
Sleep 500ms
Enter
Sleep 1.5s
Enter
Sleep 3s

# The checkout flow, fullscreen, running.
Ctrl+Space
Sleep 700ms
Type "i"
Sleep 1.2s
Ctrl+Space
Sleep 500ms
Type "f"
Sleep 1s
Enter
Sleep 1.5s
Type "r"
Sleep 7s
