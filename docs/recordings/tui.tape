# senda TUI walkthrough — drives the REAL binary against docs/recordings/senda-api,
# backed by the built-in mock server on :8787 (record.sh starts/stops it).
#
# Produces docs/screenshots/tui/walkthrough.gif plus the gallery stills.
# NOTE: Sleep timings are best-effort — tune them on the first run so each
# Screenshot lands on the intended state. Requests are opened via the command
# palette (Ctrl+K) rather than tree navigation, which is far more robust.

Require ttyd

Output docs/screenshots/tui/walkthrough.gif

Set Shell bash
Set FontSize 16
Set Width 1240
Set Height 760
Set Padding 16
Set TypingSpeed 55ms
Set Theme "TokyoNight"

# launch (hidden so the gif opens on the TUI, not the shell prompt).
# Reset PS1 first so the recording NEVER shows your real user@host or home path.
Hide
Type "export PS1='$ ' COLORTERM=truecolor TERM=xterm-256color && clear"
Enter
Type "./bin/senda tui -collection docs/recordings/senda-api -env prod"
Enter
Sleep 2.5s
Show

# open "List users" via the command palette, then send it.
# NOTE: the palette filters by substring of "<METHOD> <filename>" (e.g.
# "GET list-users"), so the query must be a literal substring — hyphen, not space.
Sleep 800ms
Ctrl+K
Sleep 500ms
Type "list-users"
Sleep 500ms
Enter
Sleep 800ms
Type "s"
Sleep 1.5s
Screenshot docs/screenshots/tui/01-three-pane.png

# response: Tests then Timing (focus response pane, then tab digits)
Tab@300ms 2
Type "4"
Sleep 1s
Type "5"
Sleep 800ms
Screenshot docs/screenshots/tui/02-tests-timing.png
Type "1"
Sleep 500ms

# command palette — fuzzy filter
Ctrl+K
Sleep 600ms
Type "user"
Sleep 600ms
Down
Sleep 600ms
Screenshot docs/screenshots/tui/03-command-palette.png
Escape
Sleep 500ms

# environments manager
Ctrl+E
Sleep 800ms
Screenshot docs/screenshots/tui/04-environments.png
Escape
Sleep 500ms

# environment cycle (motion for the gif)
Type "]"
Sleep 600ms
Type "["
Sleep 600ms

# code export / codegen
Type "x"
Sleep 800ms
Screenshot docs/screenshots/tui/05-code-export.png
Escape
Sleep 500ms

# layouts: stacked, then distraction-free focus mode
Ctrl+\
Sleep 800ms
Screenshot docs/screenshots/tui/07-stacked-layout.png
Ctrl+\
Sleep 800ms
Screenshot docs/screenshots/tui/08-focus-mode.png
Ctrl+\
Sleep 600ms

# done
Type "q"
Sleep 500ms
