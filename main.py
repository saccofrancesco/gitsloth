# Importing libraries
import click
import subprocess
import os
import openai
from rich.console import Console
from rich.panel import Panel
from rich.prompt import Confirm
from rich.traceback import install

# Enable Rich formatted tracebacks for better debugging output
install()

# Global Rich console used throughout the application
console: Console = Console()


# Helper function to establish if the tool is being called inside an initted .git repo
def is_git_repo() -> bool:
    """
    Check whether the current directory is inside a Git repository.

    Returns:
        bool: True if inside a Git repository, otherwise False.
    """
    try:
        subprocess.run(
            ["git", "rev-parse", "--is-inside-work-tree"],
            check=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        return True
    except subprocess.CalledProcessError:
        return False


# Helper function called if inside a .git repo that searches for staged files
def get_staged_diff() -> str:
    """
    Retrieve the staged Git diff.

    This represents the changes currently added to the staging
    area via `git add`.

    Returns:
        str: The staged diff content.
        Returns an empty string if no changes are staged.
    """
    result: subprocess.CompletedProcess[str] = subprocess.run(
        ["git", "diff", "--cached"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    return result.stdout.strip()


# Validates the environment and returns the staged diff, or prints an error and returns None
def get_diff() -> str | None:
    """
    Validate the Git environment and return the staged diff.

    Checks that the current directory is inside a Git repository and
    that there are changes staged for commit. Prints a descriptive
    error message and returns None if either check fails.

    Returns:
        str | None:
            The staged diff string, or None if validation failed.
    """

    if not is_git_repo():
        console.print("\n[bold red]✗ Not inside a .git repository.[/]\n")
        return None

    diff: str = get_staged_diff()

    if not diff:
        console.print("\n[bold red]✗ No staged changes found.[/]\n")
        return None

    return diff


# Shared Conventional Commits rules injected into every generation prompt
CONVENTIONAL_COMMIT_RULES: str = """
1. Use the Conventional Commits format:
<type>(optional scope): <short summary>

2. Allowed types:
feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert

3. The summary must:
- Be in lowercase
- Not end with a period
- Be concise (max 72 characters)
- Use imperative mood (e.g., "add", "fix", not "added", "fixes")

4. If the change is breaking, add:
- An exclamation mark after the type/scope (e.g., feat!:)
- A "BREAKING CHANGE:" section in the footer

5. ONLY IF additional context is helpful, include a body separated by a blank line.
"""


# Helper that builds an authenticated OpenAI client, or returns None if the key is unset
def get_openai_client() -> openai.OpenAI | None:
    """
    Build an authenticated OpenAI client from the environment.

    Returns:
        openai.OpenAI | None:
            A ready-to-use client, or None if OPENAI_API_KEY is not set.
    """

    api_key: str = os.getenv("OPENAI_API_KEY")

    if not api_key:
        return None

    return openai.OpenAI(api_key=api_key)


# Generates a commit message suggestion from a staged diff
def generate_commit_message(diff: str) -> str | None:
    """
    Generate a Conventional Commit message suggestions based on a Git diff.

    Args:
        diff (str):
            The staged Git diff.

    Returns:
        str | None:
            A string containing the suggested message, or None if the API key
            is not configured.
    """

    client: openai.OpenAI | None = get_openai_client()

    if client is None:
        return None

    prompt: str = f"""
        You are an expert software engineer that writes precise commit messages.
        Follow the Conventional Commits specification.
        {CONVENTIONAL_COMMIT_RULES}
        Task: generate a properly formatted commit message
        Changes:
        {diff}
        Return ONLY the commit message.
    """

    response = client.chat.completions.create(
        model="gpt-4o-mini",
        temperature=0.2,
        messages=[
            {"role": "system", "content": "You write excellent git commit messages."},
            {"role": "user", "content": prompt},
        ],
    )

    return response.choices[0].message.content.replace("```", "").strip()


# Helper function which given a commit message does the commit process
def create_commit(message: str) -> None:
    """
    Create a Git commit using the provided message.

    Args:
        message (str):
            The commit message to use.
    """

    result: subprocess.CompletedProcess[str] = subprocess.run(
        ["git", "commit", "-m", message],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )

    # If the commit failed, propagate the error
    if result.returncode != 0:
        console.print(
            Panel(result.stderr.strip(), title="Commit failed", border_style="red")
        )
        return

    # Display Git's output on success
    console.print(
        Panel(
            result.stdout.strip(),
            title="[bold green]✓ Commit created successfully[/]",
            border_style="green",
        )
    )


# Generates and applies a single commit message for staged changes
@click.command()
def main() -> None:
    """
    Generates a single commit message suggestion and asks the user
    whether it should be used for the commit.
    """

    diff: str | None = get_diff()

    if diff is None:
        return

    console.status("Generating commit message...", spinner="dots")

    with console.status("Generating commit message..."):
        message: str | None = generate_commit_message(diff)

    if message is None:
        console.print("\n[bold red]✗ OpenAI API key not set.[/]\n")
        return

    console.print(Panel(message, title="Proposed commit message", border_style="cyan"))

    if not Confirm.ask("Accept and commit with this message?"):
        console.print()
        return

    create_commit(message)


# Program main entry
if __name__ == "__main__":
    main()
