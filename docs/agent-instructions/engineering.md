# Engineering And Commit Rules

## Implementation

- Keep business and configuration logic in UI-free Go packages.
- Write a failing behavioral test, implement the smallest correction, then run
  the relevant package and integration checks.
- Use temporary HOME directories and fake SSH processes for tests. Never use
  the user's configuration as a fixture.
- Verify SSH writes with the actual resolved configuration before and after a
  confirmed transaction.

## Validation

Run the affected checks plus the phase-close suite:

```text
make test
make lint
make test-e2e
make gate-visual-regression
```

## Commits

- Commit one buildable logical change with its tests and documentation.
- Let normal hooks run; never use `--no-verify`.
- Preserve unrelated dirty or untracked work. Do not reset, clean, or overwrite
  it.
