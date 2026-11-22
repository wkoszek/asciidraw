package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestOSC52(t *testing.T) {
	old := osc52Out
	t.Cleanup(func() { osc52Out = old })

	osc52Out = nil
	if osc52Write("x") == nil {
		t.Error("with no terminal, OSC 52 should report failure")
	}

	var buf bytes.Buffer
	osc52Out = &buf
	text := "╭──╮\n╰──╯"
	if err := osc52Write(text); err != nil {
		t.Fatal(err)
	}
	want := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\a"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}

}

func TestOSC52Fallback(t *testing.T) {
	old := osc52Out
	t.Cleanup(func() { osc52Out = old })
	var buf bytes.Buffer
	osc52Out = &buf
	failing := orOSC52(func(string) error { return errNoClipboard })
	if err := failing("hi"); err != nil || buf.Len() == 0 {
		t.Errorf("a failed copy should fall back to OSC 52: err=%v out=%q", err, buf.String())
	}
}

func TestPickClipboardOverSSH(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "")
	t.Setenv("SSH_TTY", "/dev/pts/0")
	t.Setenv("DISPLAY", ":0")
	old := osc52Out
	t.Cleanup(func() { osc52Out = old })
	var buf bytes.Buffer
	osc52Out = &buf
	w, r := pickClipboard()
	if err := w("hi"); err != nil || buf.Len() == 0 {
		t.Errorf("over SSH, copy should go through OSC 52: err=%v out=%q", err, buf.String())
	}
	if _, err := r(); err == nil {
		t.Error("over SSH, paste should report no system clipboard")
	}
}
