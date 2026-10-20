# Skill Manager

Skill Manager keeps one copy of each agent skill at `~/.skills/<name>/SKILL.md`.

This repository has the skill name rules and the `SKILL.md` reader. Names match `^[a-z0-9][a-z0-9-]{0,63}$`. The reader keeps unknown frontmatter keys, including their order, comments, and quoting. The tree hash uses the sorted path, the file type, the permission, and the content.

## Run the tests

Install Go 1.22 or newer.

```bash
go test -race ./...
go vet ./...
golangci-lint run
```

CI uses golangci-lint 2.15.0.
