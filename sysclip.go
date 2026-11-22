package main

import (
	"encoding/base64"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// The system clipboard on every platform. Each one names the programs to try,
// in order; the first found on PATH wins. Over SSH, or with no program at all,
// copying falls back to OSC 52: an escape sequence that asks the terminal
// itself to set the clipboard. Most modern terminals honor it, including over
// SSH. asciidraw only writes it, so paste then uses asciidraw's own copy.

type clipCmd struct {
	copy, paste []string
}

// Windows' clip.exe garbles UTF-8, and box-drawing characters are UTF-8, so
// go through PowerShell and hand it the text as base64.
const psPaste = "[Console]::OutputEncoding=[Text.Encoding]::UTF8; Get-Clipboard -Raw"

func psCopy(ps string) func(string) error {
	return func(s string) error {
		b64 := base64.StdEncoding.EncodeToString([]byte(s))
		script := "Set-Clipboard -Value ([Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('" + b64 + "')))"
		return exec.Command(ps, "-NoProfile", "-NonInteractive", "-Command", script).Run()
	}
}

// osc52Out is the terminal, set by main once the screen is up. Nil in tests.
var osc52Out io.Writer

var errNoClipboard = errors.New("no system clipboard")

func osc52Write(s string) error {
	if osc52Out == nil {
		return errNoClipboard
	}
	// Plain, not wrapped for tmux: tmux with set-clipboard on takes OSC 52
	// itself, while a wrapped one needs allow-passthrough, which is off by
	// default.
	_, err := io.WriteString(osc52Out, "\x1b]52;c;"+base64.StdEncoding.EncodeToString([]byte(s))+"\a")
	return err
}

// orOSC52 falls back to OSC 52 when the clipboard program fails.
func orOSC52(copy func(string) error) func(string) error {
	return func(s string) error {
		if err := copy(s); err != nil {
			return osc52Write(s)
		}
		return nil
	}
}

func runCopy(argv []string, s string) error {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = strings.NewReader(s)
	return cmd.Run()
}

func runPaste(argv []string) (string, error) {
	out, err := exec.Command(argv[0], argv[1:]...).Output()
	return string(out), err
}

func onPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// pickClipboard chooses the copy and paste functions for this machine.
func pickClipboard() (func(string) error, func() (string, error)) {
	noPaste := func() (string, error) { return "", errNoClipboard }

	switch {
	case os.Getenv("SSH_TTY") != "" || os.Getenv("SSH_CONNECTION") != "":
		// A clipboard program here would set the remote machine's clipboard,
		// not the one in front of the user.
		return osc52Write, noPaste
	case runtime.GOOS == "darwin":
		return orOSC52(func(s string) error { return runCopy([]string{"pbcopy"}, s) }),
			func() (string, error) { return runPaste([]string{"pbpaste"}) }
	case runtime.GOOS == "windows":
		return orOSC52(psCopy("powershell")),
			func() (string, error) {
				return runPaste([]string{"powershell", "-NoProfile", "-NonInteractive", "-Command", psPaste})
			}
	case os.Getenv("WSL_DISTRO_NAME") != "" && onPath("powershell.exe"):
		return orOSC52(psCopy("powershell.exe")),
			func() (string, error) {
				s, err := runPaste([]string{"powershell.exe", "-NoProfile", "-NonInteractive", "-Command", psPaste})
				return strings.ReplaceAll(s, "\r\n", "\n"), err
			}
	}

	var tools []clipCmd
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		tools = append(tools, clipCmd{[]string{"wl-copy"}, []string{"wl-paste", "--no-newline"}})
	}
	if os.Getenv("DISPLAY") != "" {
		tools = append(tools,
			clipCmd{[]string{"xclip", "-selection", "clipboard"}, []string{"xclip", "-selection", "clipboard", "-o"}},
			clipCmd{[]string{"xsel", "--clipboard", "--input"}, []string{"xsel", "--clipboard", "--output"}})
	}
	for _, t := range tools {
		if onPath(t.copy[0]) && onPath(t.paste[0]) {
			return orOSC52(func(s string) error { return runCopy(t.copy, s) }),
				func() (string, error) { return runPaste(t.paste) }
		}
	}
	return osc52Write, noPaste
}
