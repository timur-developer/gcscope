Output docs/assets/demo_features.gif

Set Shell "bash"
Set FontFamily "JetBrains Mono"
Set FontSize 18
Set Width 1920
Set Height 1080
Set Framerate 30
Set Theme "Catppuccin Mocha"
Set Padding 8
Set Margin 10
Set MarginFill "#0b0b12"
Set BorderRadius 8
Set CursorBlink false
Set TypingSpeed 70ms

Set WaitTimeout 20s

Hide
Type "mkdir -p /tmp/gcviz-demo && cp ./bin/gcviz-linux /tmp/gcviz-demo/gcviz && chmod +x /tmp/gcviz-demo/gcviz"
Enter
Type "export PATH=/tmp/gcviz-demo:$PATH"
Enter
Type "clear"
Enter
Type "gcviz lab alloc"
Enter
Wait+Screen /LIVE/
Wait+Screen /GC cycles total: [0-9]/
Sleep 8s
Show

Sleep 4s

Type "?"
Sleep 1200ms
Type "?"

Sleep 1500ms

Type "+"
Sleep 900ms
Type "+"
Sleep 900ms
Ctrl+Up
Sleep 900ms
Ctrl+Up
Sleep 900ms
Ctrl+Down
Sleep 900ms

Type "z"
Sleep 900ms
Type "+"
Sleep 900ms
Type "+"
Sleep 900ms
Ctrl+Up
Sleep 900ms
Ctrl+Down
Sleep 900ms

Type "["
Sleep 900ms
Type "]"
Sleep 900ms

Type "g"
Sleep 1200ms
Type "g"
Sleep 1200ms

Type "l"
Sleep 1200ms
Type "l"
Sleep 1200ms

Type "r"
Sleep 1200ms

Space
Sleep 1500ms
Left 2
Sleep 1000ms
Right 2
Sleep 1000ms
Space
Sleep 1200ms

Type "s"
Sleep 1500ms

Type "q"
Sleep 1s
