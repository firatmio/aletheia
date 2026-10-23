# The README demo, assets/demo.gif. Recorded with VHS in Docker, from the repo root:
#   GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/demo/aletheia ./cmd/aletheia
#   docker run --rm -v "$PWD:/vhs" -v "$PWD/dist/demo:/opt/aletheia" ghcr.io/charmbracelet/vhs:v0.10.0 assets/demo.tape

Output assets/demo.gif

Set Shell bash
Set FontSize 17
Set Width 1280
Set Height 500
Set Padding 32
Set TypingSpeed 35ms
Set Theme { "name": "aletheia", "background": "#0a0a0a", "foreground": "#e5e5e5", "cursor": "#3fc69a", "selection": "#2a2a2a", "black": "#0a0a0a", "red": "#f87171", "green": "#3fc69a", "yellow": "#e3a14a", "blue": "#8fb4ff", "magenta": "#c4a7e7", "cyan": "#7fd1c7", "white": "#e5e5e5", "brightBlack": "#6b6b6b", "brightRed": "#f87171", "brightGreen": "#3fc69a", "brightYellow": "#e3a14a", "brightBlue": "#8fb4ff", "brightMagenta": "#c4a7e7", "brightCyan": "#7fd1c7", "brightWhite": "#f5f5f5" }

Hide
Type `export PATH=/opt/aletheia:$PATH PS1='\[\e[90m\]shop\[\e[0m\] $ ' && mkdir -p /tmp/shop && cd /tmp/shop && printf 'module shop\n\ngo 1.24\n' > go.mod && printf 'package shop\n\n// MaxDiscount caps every discount.\nconst MaxDiscount = 0.30\n\nfunc Discount(price, rate float64) float64 {\n\treturn price * min(rate, MaxDiscount)\n}\n' > pricing.go && aletheia init > /dev/null && clear`
Enter
Sleep 500ms
Show

Type "grep -n MaxDiscount pricing.go"
Enter
Sleep 2s

Type `aletheia remember "Discounts are capped at 30%" --anchor 'contains:pricing.go::MaxDiscount = 0\.30'`
Enter
Sleep 2.5s

Type `aletheia recall "how big can a discount be"`
Enter
Sleep 2.5s

Type "mv pricing.go discount.go          # someone moves the file"
Enter
Sleep 600ms
Type `aletheia recall "how big can a discount be"`
Enter
Sleep 3s

Type "sed -i 's/0.30/0.50/' discount.go  # and raises the cap"
Enter
Sleep 600ms
Type `aletheia recall "how big can a discount be"`
Enter
Sleep 2s
Type "aletheia verify"
Enter
Sleep 4s
