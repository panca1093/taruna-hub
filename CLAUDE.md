@AGENTS.md

## Claude Code specifics

- **Go toolchain:** the shell's `GOROOT` is wrong on this machine. `api/Makefile` already sets it. For direct `go` commands, prefix `export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec &&`. An LSP `package unsafe is not in std` error is noise.
- **Before UI work,** read `requirement/DESIGN.md` and the relevant `web/components/` files. Use the `frontend-design` skill only for net-new surfaces, and keep its output within `requirement/DESIGN.md` tokens and patterns.
- **Browser verification:** start the web app via `.claude/launch.json` (`bimbel-web`). Stop `next dev` before running `npm run build`.
- **Parallel subagents:** give each agent a non-overlapping set of files. Before using `isolation: "worktree"`, check `git status`, because worktrees only contain committed work.
- **Decision gates:** if a task touches an unresolved D01–D16 policy, ask with the decision ID instead of picking a default.
