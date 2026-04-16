## Summary

What does this PR change and why?

## Type of change

- [ ] Bug fix
- [ ] New feature
- [ ] Refactor (no behavior change)
- [ ] Documentation
- [ ] Test improvement

## DDD checklist

- [ ] Domain has zero external imports
- [ ] Aggregates enforce invariants (unexported fields, defensive copies)
- [ ] Port interfaces live in `domain/`
- [ ] No domain -> infrastructure dependencies

## Quality checklist

- [ ] `make check` passes (fmt + lint + test + security)
- [ ] `go test ./... -race` passes
- [ ] Tests added/updated for behavior changes
- [ ] README and CLAUDE.md updated if public API changed
- [ ] Commit messages follow Conventional Commits

## Related issues

Closes #
