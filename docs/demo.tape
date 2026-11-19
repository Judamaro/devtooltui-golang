# Demo de devtooltui para el README. Se genera con el workflow "Demo"
# (charmbracelet/vhs-action) o en local con: vhs docs/demo.tape
Output docs/demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 1280
Set Height 720
Set Padding 16
Set TypingSpeed 40ms
Set Theme "Catppuccin Mocha"

Hide
Type "go build -o /tmp/devtooltui ./cmd/devtooltui && export PATH=/tmp:$PATH && clear"
Enter
Sleep 8s
Show

# CLI componible
Type `echo '{"b":1,"a":[1,2]}' | devtooltui json-format --sort-keys`
Enter
Sleep 1.5s
Type `devtooltui base64 decode <<< 'aG9sYSBtdW5kbw=='`
Enter
Sleep 1.5s
Type "clear"
Enter

# TUI
Type "devtooltui"
Enter
Sleep 1s
Tab
Tab
Type `{"id":1,"items":["a","b"],"ok":true}`
Sleep 1.5s
Ctrl+P
Sleep 500ms
Type "b64"
Sleep 800ms
Enter
Sleep 500ms
Tab
Tab
Type "hola mundo"
Sleep 1s
Ctrl+R
Sleep 1.5s
Ctrl+P
Type "diff"
Enter
Sleep 500ms
Tab
Tab
Type "uno"
Enter
Type "dos"
Tab
Type "uno"
Enter
Type "DOS"
Sleep 2s
Ctrl+C
