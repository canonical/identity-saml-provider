---
name: lint-markdown
description: >
  Lints Markdown files in this repository against style rules using
  markdownlint-cli. WHEN: create markdown file, edit markdown file,
  update markdown file, write markdown file, create new .md file, edit
  existing .md file, generate markdown content, run markdownlint, run
  markdownlint-cli, fix markdown lint errors.
license: AGPL-3.0-only
metadata:
  author: Canonical
  version: "1.0.0"
  summary: >
    Lints Markdown files in this repository against style rules using
    markdownlint-cli.
  tags:
    - markdown
    - lint
    - documentation
    - formatting
---

# Lint Markdown

All Markdown (`*.md`) files in this repository must comply with the style
rules configured in `.markdownlint.yaml` at the repository root.

## Application and Scope

This skill is triggered whenever any Markdown file in this repository is
created or modified, excluding those listed in `.markdownlintignore`.

## Workflow

Agents must execute the following steps in sequence:

1. **Check Exemption**: Consult `.markdownlintignore` at the repository root.
   If the target file is listed, proceed with editing but match the file's
   existing formatting style. If not listed, proceed to step 2.
2. **Execute Validation**: Before making changes, run the validation tool to
   identify existing violations:

   ```bash
   npx --yes markdownlint-cli "<path-to-file>"
   ```

3. **Perform Surgical Edits**: Apply changes while respecting the rules defined
   in `.markdownlint.yaml`. Avoid introducing unrelated formatting
   modifications.
4. **Remediate**: If violations are found, resolve them in place. The `--fix`
   flag may be used as a first pass, but must always be verified by a strict
   run without the flag:

   ```bash
   npx --yes markdownlint-cli --fix "<file>" && \
     npx --yes markdownlint-cli "<file>"
   ```

5. **Verify Clean Pass**: The task is complete only when the final validation
   run produces empty output and exits with `0`.

## Validation Recipes

### Validate a Single File

```bash
npx --yes markdownlint-cli "docs/example.md"
```

### Validate Multiple Files

```bash
npx --yes markdownlint-cli "docs/foo.md" "plan/bar.md"
```

### Validate the Entire Workspace

This command automatically respects `.markdownlintignore` configurations:

```bash
npx --yes markdownlint-cli "**/*.md"
```

### Auto-fix and Re-verify

```bash
npx --yes markdownlint-cli --fix "<file>" && \
  npx --yes markdownlint-cli "<file>"
```

## Definition of Done

A Markdown task is complete only when:

1. Every edited, non-ignored Markdown file passes `npx markdownlint-cli` with
   an exit code of `0` and empty output.
2. The content is accurate, high-quality, and addresses the user's
   requirements.
3. Edits are surgical and do not introduce unrelated stylistic changes to
   other sections of the document.
