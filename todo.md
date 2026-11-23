# asciidraw todo

Stuff to do to get it where I'd like it to be.

## 1. Housekeeping

- [x] Make it a git repo: Gitea (tig.koszek.com), mirrored to GitHub.
- [x] Add `.github/repo.diaml`.
- [x] Write a README: what it is, install, keys, file formats.
- [ ] Second outside review of the README with `agy` (its quota ran out on
      2026-11-20; a cold-read agent stood in).
- [x] Pick a license: BSD 2-Clause.

## 2. UI widget palette

Stamp ready-made pieces instead of building them from boxes and text. Each
is one object you can move, resize and relabel. Put them in the ^T menu
under a third heading, "Widgets".

- [x] Button `[ OK ]`
- [x] Checkbox `[x]` and radio button `(•)`
- [x] Input field `[__________]`
- [x] Dropdown `[ Choose  v ]`
- [x] Tabs
- [x] Scrollbar
- [x] Progress bar `[████░░░░]`
- [x] Window with a title bar

## 3. Select several shapes

- [x] Drag a rectangle around shapes to select them.
- [x] Shift-click to add a shape to the selection.
- [x] Move, copy, cut and delete the whole selection.
- [x] Group, so a dialog and its buttons move as one piece.

## 4. Arrows that stay attached

- [x] An arrow that starts or ends on a box's edge stays attached when the
      box moves.

## 5. Labels that belong to boxes

- [x] Text typed inside a box moves with the box.
- [x] Center a label in its box.
- [x] Grow a box to fit its label.

## 6. Clean crossings

- [x] Where lines meet, draw `┼ ├ ┬` (or `+` in ASCII) instead of letting
      one shape overwrite the other.

## 7. Smaller things

- [x] Bring to front and send to back, maybe `[` and `]`.
- [x] Reorder shapes by dragging rows in the panel.
- [x] Duplicate in place, with an offset, in one key.
- [x] Real text editing: arrow keys, and typing in the middle of a line.
- [x] A canvas bigger than the window, with scrolling.
- [x] `?` opens a help screen with every key.
- [x] Fill patterns `░▒▓` for shaded areas.
- [x] Autosave, so a crash or a closed window loses nothing.

## 8. Drawings inside docs

- [ ] ^E updates a drawing inside an existing Markdown file, between
      marker comments, instead of writing a separate `.md`.

## 9. Release it everywhere

The config is in place (`.goreleaser.yaml`, `scripts/macos-sign.sh`). What's
left is mostly accounts, repos and testing on real machines.

### Ship v0.1.0

- [x] Commit the palette work that is still uncommitted. GoReleaser won't
      release from a dirty tree.
- [x] Re-tag v0.1.0 at that commit (`git tag -f -a v0.1.0 -m v0.1.0`). The
      tag hasn't been pushed, so moving it is safe.
- [x] Make `wkoszek/asciidraw` public on GitHub. Set `visibility: public` in
      `.github/repo.diaml`.
- [x] `git push tig main v0.1.0`, then check that the mirror carried both the
      commit and the tag to GitHub.
- [x] `make release`.
- [x] Check the release page: 13 archives (not the 15 expected), deb,
      rpm, apk, Arch packages, checksums.txt.

### Install it on real machines

Each of these has only been built, not run.

- [ ] macOS: `brew install wkoszek/tap/asciidraw` on Apple silicon and on
      Intel. It must start with no Gatekeeper warning.
- [ ] Linux: `brew install` through Homebrew on Linux (it installs casks).
- [ ] Debian and Ubuntu `.deb`, Fedora `.rpm`, Alpine `.apk`, Arch
      `.pkg.tar.zst`. Each can run in a Docker container.
- [ ] `go install github.com/wkoszek/asciidraw@latest`, then
      `asciidraw --version` prints v0.1.0.
- [ ] Windows: unzip and run in Windows Terminal. (The README now names
      the autosave folder, `%LocalAppData%\asciidraw\recover`.)
- [ ] FreeBSD and OpenBSD in a VM.

### Mouse and color in other terminals

asciidraw has only been used in Ghostty.

- [ ] Drag, click, Shift+drag and scroll in iTerm2, Terminal.app, kitty,
      WezTerm, Alacritty, GNOME Terminal, Konsole, Windows Terminal.
- [ ] Inside tmux and GNU screen.
- [ ] Terminal.app has no 24-bit color. See how hex colors look there.
- [ ] Add a "Terminals" table to the README with what works where.

### Clipboard on real systems

So far only unit tests cover `sysclip.go`.

- [ ] Linux under X11 (xclip, xsel) and under Wayland (wl-copy).
- [ ] Windows: PowerShell copy and paste, and box-drawing characters
      surviving the trip.
- [ ] WSL with `powershell.exe`.
- [ ] Over SSH: OSC 52 in Ghostty, iTerm2, kitty, WezTerm, Windows Terminal.
- [ ] tmux with `set -g set-clipboard on`, local and over SSH.

### Turn on the extra channels

- [ ] Scoop: create the public repo `wkoszek/scoop-bucket`, then release
      with `PUBLISH_SCOOP=1`. Add `scoop install` to the README.
- [ ] winget: fork `microsoft/winget-pkgs` to `wkoszek/winget-pkgs`, then
      release with `PUBLISH_WINGET=1`. Microsoft reviews the first PR by hand.
      Add `winget install wkoszek.asciidraw` to the README once it merges.
- [ ] AUR: make an account on aur.archlinux.org, add an SSH key, and store
      the key in 1Password. Have `make release` read it with `opc`. Release
      with `AUR_KEY` set. Add `yay -S asciidraw-bin` to the README.
- [ ] Nix: send a `buildGoModule` package to nixpkgs. That beats a personal
      NUR repo and needs no Nix on this Mac.
- [ ] MacPorts: a Portfile using the Go port group.
- [ ] Chocolatey, if Windows users ask. Scoop and winget cover most of them.

### Get into the distros themselves

These take a maintainer and weeks of review. Most need the project to look
alive first: a few releases, stars, an issue tracker.

- [ ] homebrew-core: needs about 75 stars or 30 forks and a stable release.
      homebrew-core wants a formula that builds from source, not a cask.
      Write one with `go build` and a test block.
- [ ] Alpine: a package in `aports` under testing. This is the easiest
      official distro.
- [ ] Arch: move from AUR to the extra repo once a packager adopts it.
- [ ] Debian, and through it Ubuntu: file an ITP bug and package it with the
      Debian Go team. tcell is already in Debian. Debian 13 ships Go 1.24,
      which matches `go.mod`.
- [ ] Fedora: a package review request. EPEL after that.
- [ ] FreeBSD ports, OpenBSD ports, pkgsrc for NetBSD.
- [ ] Watch https://repology.org/project/asciidraw to see who has it.

### What packagers will ask for

- [x] A man page, `asciidraw.1`, in mdoc. It ships in the archives, the cask,
      the deb, rpm, apk and Arch packages, and the AUR package.
- [x] CHANGELOG.md. `make release` uses the tag's section as the release
      notes.
- [ ] A source tarball with vendored modules. Debian, Fedora and nixpkgs
      build offline.
- [ ] Sign checksums.txt (cosign or GPG) and attach an SBOM.
- [x] More targets: linux/riscv64, linux/386, linux/loong64. Debian and
      Alpine build them.
- [ ] Run the tests on macOS and Windows, not only Linux. The Gitea runner is
      Linux only.

### Let people find it

- [ ] A demo GIF in the README, made with `vhs`.
- [x] GitHub description and topics.
- [ ] A GitHub social preview image.
- [ ] Decide where public bug reports go. tig is private, so either open
      GitHub issues or say where to write.
- [ ] Post it: Show HN, r/commandline, lobste.rs, Terminal Trove, the
      awesome-tui and awesome-cli-apps lists.

## Formats

Exports to:

- [x] SVG
- [x] PNG
- [x] JPG
- [ ] PIC (manual pages)
- [ ] Graphviz
- [ ] TeX/LaTeX
- [x] HTML

## Sumcommands

We need to add:

- [x] asciidraw export <file> <output>
- [x] asciidraw convert <file> <output>
