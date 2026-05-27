Output docs/assets/demo_launch_app.gif

Set Shell "bash"
Set FontFamily "JetBrains Mono"
Set FontSize 16
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
Sleep 2s
Show

Sleep 4s
Type "q"
Sleep 1s
