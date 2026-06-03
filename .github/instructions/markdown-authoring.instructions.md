---
description: 'Enforces markdownlint rules from .markdownlint.yaml for all Markdown files. Ensures every generated or modified Markdown document passes markdownlint-cli validation without human intervention.'
applyTo: '**/*.md'
---

# Markdown Authoring Standards

All Markdown files in this repository **MUST** conform to
the rules defined in `.markdownlint.yaml` at the
repository root. Violations are not acceptable—every edit
must produce lint-clean output as verified by
[markdownlint-cli](https://github.com/igorshubovych/markdownlint-cli).

## Scope

These rules apply to every `*.md` file in the repository,
**EXCEPT** files declared in `.markdownlintignore` at the
repository root. Do **NOT** apply or enforce these rules
on ignored files.

## Validation Requirement

After every Markdown file creation or modification, run
the following command to verify compliance:

```bash
npx markdownlint-cli "path/to/file.md"
```

If any violations are reported, fix them immediately and
re-validate. Do not present the result to the user until
the file passes with zero violations.

## Document Structure

### Headings

- **Single top-level heading (MD025):** Each document must
  contain exactly one `#` (H1) heading. If YAML
  frontmatter includes a `title` field, that satisfies
  the requirement.
- **Incremental heading levels (MD001):** Heading levels
  must increment by exactly one level at a time. Never
  skip from `##` to `####`.
- **Consistent heading style (MD003):** Use a single
  heading style consistently throughout the document.
  Prefer ATX-style (`# Heading`).
- **Blank lines around headings (MD022):** Insert exactly
  one blank line above and one blank line below every
  heading.
- **Left-aligned headings (MD023):** Headings must begin
  at column 1—no leading whitespace.
- **No duplicate headings (MD024):** Heading text must be
  unique across the entire document (not limited to
  siblings).
- **No trailing punctuation in headings (MD026):** Do not
  end headings with `.` `,` `;` `:` `!` or their
  full-width equivalents.
- **Exactly one space after `#` (MD018/MD019):** Use a
  single space between the `#` markers and the heading
  text.
- **First line must be a heading (MD041):** The first
  non-frontmatter line must be a level-1 heading.

### Blank Lines

- **No multiple consecutive blank lines (MD012):** Use at
  most one consecutive blank line.
- **Blank lines around fenced code blocks (MD031):**
  Fenced code blocks must be preceded and followed by a
  blank line, including within list items.
- **Blank lines around lists (MD032):** Lists must be
  surrounded by blank lines.

### Line Length

- **80-character line limit (MD013):** Prose lines must
  not exceed 80 characters. This rule excludes code
  blocks, tables, and headings.

## Lists

- **Consistent unordered list style (MD004):** Use the
  same list marker (`-`, `*`, or `+`) throughout the
  document.
- **2-space unordered list indentation (MD007):** Indent
  nested unordered list items by exactly 2 spaces.
- **Consistent list indentation (MD005):** Items at the
  same nesting level must use identical indentation.
- **Single space after list markers (MD030):** Place
  exactly one space between the list marker and the item
  text for both ordered and unordered lists.
- **Ordered list prefix style (MD029):** Use either
  all-ones (`1.`) or sequential numbering—be consistent
  within each list.

## Code Blocks

- **Specify a language for fenced code blocks (MD040):**
  Every fenced code block must declare a language
  identifier (e.g., ` ```go `, ` ```bash `, ` ```yaml `).
- **Consistent code block style (MD046):** Use a single
  code block style (fenced or indented) throughout the
  document. Prefer fenced blocks.
- **Consistent code fence style (MD048):** Use the same
  fence character (backticks or tildes) throughout the
  document. Prefer backticks.
- **No hard tabs (MD010):** Use spaces instead of tab
  characters, including inside code blocks.

## Inline Formatting

- **No spaces inside emphasis markers (MD037):** Write
  `*emphasis*`, not `* emphasis *`.
- **No spaces inside code spans (MD038):** Write
  `` `code` ``, not `` ` code ` ``.
- **No spaces inside link text (MD039):** Write
  `[text](url)`, not `[ text ](url)`.
- **No emphasis as heading substitute (MD036):** Do not
  use bold or italic text as a heading replacement.
- **Consistent emphasis style (MD049):** Use a single
  emphasis marker style (`*` or `_`) throughout the
  document.
- **Consistent strong style (MD050):** Use a single
  strong marker style (`**` or `__`) throughout the
  document.

## Links and Images

- **No bare URLs (MD034):** Wrap URLs in angle brackets
  (`<https://example.com>`) or use a Markdown link.
- **No empty links (MD042):** Every link must have a
  non-empty destination.
- **No reversed link syntax (MD011):** Use `[text](url)`,
  not `(text)[url]`.
- **Valid link fragments (MD051):** Internal anchor links
  must reference headings that exist in the document.
- **Defined reference labels (MD052):** Reference-style
  links and images must use labels that are defined
  elsewhere in the document.
- **No unused reference definitions (MD053):** Remove
  reference definitions that are not used by any link or
  image.
- **Images must have alt text (MD045):** Every image must
  include descriptive alternate text.

## Tables

- **Consistent table pipe style (MD055):** Use the same
  pipe style throughout the document.
- **Correct column count (MD056):** Every row in a table
  must have the same number of columns.

## Blockquotes

- **Single space after `>` (MD027):** Use exactly one
  space after the blockquote marker.
- **No blank lines inside blockquotes (MD028):** Do not
  insert blank lines between blockquote lines.

## Horizontal Rules

- **Consistent horizontal rule style (MD035):** Use the
  same horizontal rule syntax throughout the document.

## Whitespace

- **No trailing spaces (MD009):** Lines must not end with
  trailing whitespace (2-space line breaks are permitted).
- **File ends with a single newline (MD047):** Every file
  must end with exactly one newline character.

## Commands in Code Blocks

- **No unnecessary dollar signs (MD014):** When showing
  shell commands without their output, omit the `$`
  prefix.

## Inline HTML

- **No inline HTML (MD033):** Do not use raw HTML elements
  in Markdown. No elements are in the allow-list.

## Automated Remediation

When generating or modifying any Markdown file:

1. Apply all rules above during content generation.
2. After writing the file, execute:

   ```bash
   npx markdownlint-cli "<file_path>"
   ```

3. If violations are reported, fix every violation
   in-place and re-run validation.
4. Repeat until the file produces zero violations.
5. Do not request human review for lint violations—
   resolve them autonomously.
