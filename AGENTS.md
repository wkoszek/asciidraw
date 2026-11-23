# asciidraw

A terminal drawing tool in Go, built on tcell.

- Run `make` before you commit. It formats, vets, tests with -race and builds.
- Keep go.mod at Go 1.24.
- The help screen must fit 80×24. `TestHelpFits80x24` checks it.
- Use conventional commits (`feat:`, `fix:`, `docs:`, `chore:` and so on). `scripts/release.sh` checks them and picks the next version from them.
- Stage files by name. Never run `git add -A` or `git add .`, because private drawings must stay out of the repo.
- You MUST keep README.md consistent with the code
- You MUST keep asciidraw.1 consistent with the code
