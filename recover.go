package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Autosave. Every 30 seconds, unsaved changes go to a recovery copy in the
// cache folder, and the same happens when the window closes. The real
// file is only ever written by ^S. Opening the drawing again while a
// recovery copy exists brings the changes back. Saving, or choosing to
// throw the changes away, deletes the copy.

const autosaveEvery = 30 * time.Second

// recoverDir is a variable so tests can point it somewhere else.
var recoverDir = func() string {
	d, err := os.UserCacheDir()
	if err != nil {
		d = os.TempDir()
	}
	return filepath.Join(d, "asciidraw", "recover")
}

// recoverPath names the recovery copy for a drawing: its full path with
// the slashes swapped for %, or untitled.yaml.
func recoverPath(file string) string {
	name := "untitled.yaml"
	if file != "" {
		if abs, err := filepath.Abs(file); err == nil {
			file = abs
		}
		name = strings.ReplaceAll(file, string(filepath.Separator), "%")
	}
	return filepath.Join(recoverDir(), name)
}

// autosave writes the recovery copy if there are unsaved changes. It writes
// to a temporary name first, so a crash mid-write can't leave half a file.
func (a *app) autosave() {
	if !a.dirty {
		return
	}
	data, err := encodeDoc(a.allPages(), a.pal)
	if err != nil {
		return
	}
	p := recoverPath(a.file)
	if os.MkdirAll(filepath.Dir(p), 0o700) != nil {
		return
	}
	if os.WriteFile(p+".tmp", data, 0o600) == nil {
		os.Rename(p+".tmp", p)
	}
}

func clearRecovery(file string) { os.Remove(recoverPath(file)) }

// loadRecovery reads the recovery copy for a drawing, if there is one.
func loadRecovery(file string) ([][]object, []string, time.Time, bool) {
	p := recoverPath(file)
	info, err := os.Stat(p)
	if err != nil {
		return nil, nil, time.Time{}, false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, nil, time.Time{}, false
	}
	pages, pal, err := decodeDoc(data)
	if err != nil {
		return nil, nil, time.Time{}, false
	}
	return pages, pal, info.ModTime(), true
}

// recover swaps in the recovery copy for the drawing just opened, if any.
func (a *app) recover() {
	pages, pal, when, ok := loadRecovery(a.file)
	if !ok {
		return
	}
	a.setPages(pages)
	a.pal, a.dirty = pal, true
	what := "unsaved changes"
	if a.file == "" {
		what = "an unsaved drawing"
	}
	a.say("Recovered " + what + " from " + when.Format("Jan 2 15:04") + ". ^S keeps them; ^Q twice throws them away.")
}
