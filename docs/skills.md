---
title: Skills
---

# Skills

`mistermorph` supports “skills”: small, self-contained folders that contain a `SKILL.md` file (required) plus optional scripts/resources. Skills are discovered from a set of root directories and can be loaded into the agent prompt when skills mode is enabled.

Important: a skill is **not automatically a tool**. Skills add prompt context; tools are registered separately (e.g. `url_fetch`, `web_search`). If a skill includes scripts that you want the agent to execute, you must enable the `bash` tool (or implement a dedicated tool).

## Discovery and priority (dedupe)

Skills are discovered by scanning roots recursively for `SKILL.md`.

Default root:

1. `file_state_dir/skills` (usually `~/.morph/skills`)

You can add custom roots via `--skills-dir` when listing or running.

## Listing skills

- List: `mistermorph skills list`

## How skills are chosen (selection modes)

Skill loading is controlled by `skills.mode`:

- `off`: never load skills
- `on`: only load skills requested by config/flags and (optionally) `$SkillName` references

### On mode

You can request skills via config:

- `skills.load: ["some-skill-id", "some-skill-name"]`

If `skills.auto=true`, the agent also loads skills referenced inside your task text as `$SkillName` (e.g. “Use $google-maps-parse to extract coordinates.”).

## Installing / updating built-in skills

`mistermorph` ships some built-in skills under `assets/skills/`. To install (or update) them into your user skills directory:

- `mistermorph skills install`

By default this writes to `~/.morph/skills`.

Useful flags:

- `--dry-run`: print what would be written
- `--clean`: remove an existing skill directory before copying (destructive)
- `--dest <dir>`: install somewhere else (useful for testing)

After installation, the built-in skills are picked up automatically via the default roots.

## Installing a remote SKILL.md (single-file)

If you have a URL that points directly to a `SKILL.md` file, you can install/update it into `~/.morph/skills`:

- `mistermorph skills install "https://example.com/skill.md"`

Notes:

- The installer first prints the remote `SKILL.md` and asks for confirmation.
- Then it uses the configured LLM to review the file (treating it as untrusted) and extract any explicitly required additional downloads (e.g. `scripts/...`).
- Before writing anything, it prints a file plan + potential risks and asks for confirmation again.
- Safety: downloaded files are only written to disk; they are **not executed** during install.
- The destination folder name is exactly the `name:` in the YAML frontmatter (must match `[A-Za-z0-9_.-]+`).
- All downloaded files are written under `~/.morph/skills/<name>/` (no paths outside the skills directory).

## Using a skill

- Explicitly in a task: reference it as `$MySkillName` (works when `skills.auto=true`).
- Or add it to `skills.load` for always-on behavior.
