#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.12"
# dependencies = [
#     "openrouter",
#     "PyGithub",
#     "pathspec"
# ]
# ///

"""Script for performing agentic code review using OpenRouter AI.

This script runs in a GitHub Action, triggered by a PR comment. It fetches
the changed files in a PR, filters them using .aireviewignore, requests a
code review from OpenRouter, and posts the result as a PR comment.
"""

import os
import sys
from pathlib import Path
from typing import Optional

from github import Github
from github.PullRequest import PullRequest
from openrouter import OpenRouter
import pathspec

SYSTEM_PROMPT_TEMPLATE = """
You are a senior Go engineer reviewing a pull request. Apply these guidelines for each area:

## Review Areas

- **Code style** — formatting, comment quality, idiomatic Go patterns
- **Naming** — packages, types, variables, functions, constants
- **Error handling** — wrapping, sentinel errors, log-and-return, swallowed errors
- **Concurrency** — goroutine lifecycle, mutex usage, channel patterns, context propagation,
  data races
- **Code safety** — nil dereference, map/slice aliasing, integer overflows, uninitialized state
- **Tests** — coverage of new code, test quality, table-driven tests, use of t.Helper()
- **Performance** — unnecessary allocations, inefficient data structures, missing bounds
- **Security** — injection, auth, crypto misuse, sensitive data exposure, input validation
- **Documentation** — exported symbols, package docs, README impact
- **Observability** — logging, metrics, tracing added for new code paths
- **Modernize code** — outdated patterns replaced with Go 1.21+ idioms

## Priority

- **Blocking-first**: Security, Code safety, Error handling, Concurrency
- **Important**: Tests, Performance, Dependencies
- **Suggestion-first**: Code style, Naming, Documentation, Observability, Modernize code

## Severity Labels

- 🔴 **BLOCKING** — bug, vulnerability, data race, or correctness issue; must be fixed
  before merge.
- 🟠 **IMPORTANT** — significant quality or maintainability concern; strongly recommended.
- 🟡 **SUGGESTION** — style, naming, or minor improvement; optional but worthwhile.

Write short, concise comments. Reference the exact file and line. Explain what is wrong and why it
matters. Provide a concrete fix.
"""

USER_PROMPT_TEMPLATE = "Here is the diff to review:\n\n{diff}"


def load_ignore_spec(filepath: str = ".aireviewignore") -> Optional[pathspec.PathSpec]:
    ignore_path = Path(filepath)
    if not ignore_path.exists():
        print(f"INFO: No {filepath} file found.")
        return None

    with open(ignore_path, "r", encoding="utf-8") as f:
        lines = f.readlines()

    print(f"INFO: Loaded ignore patterns from {filepath}")
    return pathspec.PathSpec.from_lines(pathspec.patterns.GitWildMatchPattern, lines)


def fetch_pr(github_token: str, repo_name: str, pr_number: int) -> PullRequest:
    github_client = Github(github_token)
    repo = github_client.get_repo(repo_name)
    return repo.get_pull(pr_number)


def get_filtered_diff(pr: PullRequest, ignore_spec: Optional[pathspec.PathSpec]) -> str:
    diff_text = ""
    files = pr.get_files()

    for file in files:
        if ignore_spec and ignore_spec.match_file(file.filename):
            print(f"INFO: Ignoring file based on ignore patterns: {file.filename}")
            continue

        if file.patch:
            diff_text += f"--- {file.filename}\n+++ {file.filename}\n{file.patch}\n\n"
        else:
            print(f"INFO: No patch available for file: {file.filename}")

    return diff_text


def post_pr_comment(pr: PullRequest, body: str) -> None:
    pr.create_issue_comment(body)


def generate_review(diff: str, extra_instructions: str, api_key: str, model: str) -> str:
    if not diff.strip():
        return "No valid files to review based on the ignore list or empty diff."

    system_prompt = SYSTEM_PROMPT_TEMPLATE
    if extra_instructions.strip():
        system_prompt += f"\nAdditional Instructions:\n{extra_instructions}\n"

    print(f"INFO: Sending request to OpenRouter using model: {model}")

    messages = [
        {"role": "system", "content": system_prompt},
        {"role": "user", "content": USER_PROMPT_TEMPLATE.format(diff=diff)}
    ]

    try:
        with OpenRouter(api_key=api_key) as client:
            response = client.chat.send(
                model=model,
                messages=messages
            )
            if response.choices:
                return response.choices[0].message.content
            else:
                print("WARNING: No choices returned in OpenRouter response.", file=sys.stderr)
                return "Failed to generate review: OpenRouter returned an empty response."
    except Exception as e:
        print(f"ERROR: Failed to generate review from OpenRouter: {e}", file=sys.stderr)
        sys.exit(1)


def get_required_env(key: str) -> str:
    if not (value := os.environ.get(key)):
        print(f"ERROR: Required environment variable {key} is not set.", file=sys.stderr)
        sys.exit(1)

    return value


def main() -> None:
    print("INFO: Starting Agentic Code Review")

    # Load and validate configuration
    github_token = get_required_env("GITHUB_TOKEN")
    openrouter_api_key = get_required_env("OPENROUTER_API_KEY")
    repo_name = get_required_env("REPOSITORY_NAME")

    pr_number_str = get_required_env("PR_NUMBER")
    try:
        pr_number = int(pr_number_str)
    except ValueError:
        print("ERROR: PR_NUMBER environment variable is not a valid integer.", file=sys.stderr)
        sys.exit(1)

    model = os.environ.get("MODEL", "openrouter/deepseek/deepseek-v4-flash")
    extra_instructions = os.environ.get("EXTRA_INSTRUCTIONS", "")

    ignore_spec = load_ignore_spec()

    print(f"INFO: Fetching Pull Request #{pr_number} from {repo_name}")
    pr = fetch_pr(github_token, repo_name, pr_number)

    diff = get_filtered_diff(pr, ignore_spec)

    if not diff.strip():
        print("INFO: No files left to review after filtering.")
        post_pr_comment(pr, "No files required review based on `.aireviewignore`.")
        sys.exit(0)

    print("INFO: Generating code review.")
    review_comment = generate_review(diff, extra_instructions, openrouter_api_key, model)

    print("INFO: Posting code review to PR.")
    post_pr_comment(pr, review_comment)

    print("INFO: Agentic Code Review completed successfully.")


if __name__ == "__main__":
    main()
