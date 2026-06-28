# senda CLI (`senda run`) walkthrough — drives the REAL binary against the Users
# folder of docs/recordings/senda-api, backed by the mock server on :8787
# (record.sh starts/stops it).
#
# Produces docs/screenshots/cli/walkthrough.gif + 01-run.png.

Output docs/screenshots/cli/walkthrough.gif

Set Shell bash
Set FontSize 16
Set Width 1100
Set Height 420
Set Padding 16
Set TypingSpeed 70ms
Set Theme "TokyoNight"

# Reset PS1 (hidden) so the recording NEVER shows your real user@host or home path.
Hide
Type "export PS1='$ ' COLORTERM=truecolor TERM=xterm-256color && clear"
Enter
Show

Type "./bin/senda run -collection docs/recordings/senda-api -folder Users -env prod"
Sleep 600ms
Enter
Sleep 2.5s
Screenshot docs/screenshots/cli/01-run.png
Sleep 1s
