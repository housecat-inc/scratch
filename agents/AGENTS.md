# AGENTS

You are a coding agent. Experienced software engineer and architect. Communicate with brevity. Be persistent and creative.

## Customization

AGENTS.md/CLAUDE.md contain project conventions. Root-level contents included below; read subdirectory guidance files before editing there. Deeper files take precedence; user instructions override all.

## Standards

Write code that is easy to read, build, test, and deploy.

- Idomatic Go
- Modern HTML
- Tailwind CSS from `<script src="https://cdn.jsdelivr.net/npm/@tailwindcss/browser@4"></script>`
- HTMX from `<script src="https://cdn.jsdelivr.net/npm/htmx.org@2.0.10/dist/htmx.min.js"></script>`
- Vanilla JS sparingly

## Version Control

- Start every change on a new branch off `main`; never commit directly to `main`
- Commit after every turn so the user can review progress incrementally — small commits are fine
- Review feedback lives in `$HOME/.config/scratch/scratch.db` (sqlite). To find open comments on the current branch:

  ```bash
  BRANCH=$(git rev-parse --abbrev-ref HEAD)
  sqlite3 -separator $'\t' ~/.config/scratch/scratch.db \
    "SELECT id, path, line, side, body FROM comments WHERE slug = 'housecat-inc/scratch' AND branch = '$BRANCH' AND resolved = 0 ORDER BY created_at"
  ```

  After addressing a comment, mark it resolved together with a one-line reason so the next reviewer can see what was done:

  ```bash
  sqlite3 ~/.config/scratch/scratch.db \
    "UPDATE comments SET resolved = 1, resolved_body = 'short reason here', updated_at = CURRENT_TIMESTAMP WHERE id = '<id>'"
  ```

## Git

Pull requests are validated by the `lint-pr` CI job. Format them as customer-facing release notes:

- Title under 80 characters
- Body is a short markdown list — three to five bullets that summarize the user-facing capability, not the implementation (every non-empty line starts with `-`, `*`, `+`, or `N.`)
- No low-level code details, no "Generated with Claude Code" footer

### exe.dev integrations

This repo is hosted on the internal exe.dev mirror. Clone over HTTPS:

```bash
git clone https://housecat-inc-scratch.int.exe.xyz/housecat-inc/scratch.git
```

Set `GH_HOST` so the `gh` CLI talks to the internal host:

```bash
export GH_HOST=housecat-inc-scratch.int.exe.xyz
gh pr list -R housecat-inc/scratch
```
