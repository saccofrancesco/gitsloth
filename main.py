# Importing libraries
import click
import subprocess
import os
import openai
import questionary
from questionary import Style


# Custom style: bold cyan pointer, bright white selected text on a dark blue highlight
SELECTOR_STYLE: Style = Style(
    [
        ("question", "fg:#ffffff bold"),
        ("pointer", "fg:#00b4d8 bold"),
        ("highlighted", "fg:#ffffff bg:#023e8a bold"),
        ("selected", "fg:#00b4d8 bold"),
        ("answer", "fg:#00b4d8 bold"),
    ]
)


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
        click.echo("\n  ✗  Not inside a .git repository.\n")
        return None
    diff: str = get_staged_diff()
    if not diff:
        click.echo("\n  ✗  No staged changes found.\n")
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

        5. ONLY IF additional context is helpful, include a body separated by a blank line."""


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


# Generates one or more commit message suggestions from a staged diff
def generate_commit_messages(diff: str, num: int = 1) -> list[str] | None:
    """
    Generate one or more Conventional Commit message suggestions based on a Git diff.

    When NUM is 1 a tightly focused single message is returned. When NUM is
    greater than 1 each suggestion explores a different type or framing of the
    same change set. In both cases the result is a plain list of strings so
    callers handle it uniformly.

    Args:
        diff (str):
            The staged Git diff.
        num (int):
            Number of suggestions to generate. Defaults to 1.

    Returns:
        list[str] | None:
            A list of suggested commit message strings, or None if the API key
            is not set.
    """
    client: openai.OpenAI | None = get_openai_client()
    if client is None:
        return None

    if num == 1:
        task: str = (
            "generate a properly formatted commit message based on the provided changes."
        )
        output_format: str = "Return ONLY the commit message, with no extra text."
        temperature: float = 0.2
    else:
        task = (
            f"generate exactly {num} distinct commit message suggestions based on the provided changes. "
            "Each suggestion should explore a different type or angle of the same change set."
        )
        output_format = (
            f'Return ONLY the {num} messages, each separated by the delimiter "---" on its own line. '
            "Do not include numbering, labels, or any other text outside the messages and delimiters."
        )
        temperature = 0.7

    prompt: str = f"""
        You are an expert software engineer that writes precise commit messages following the Conventional Commits specification.

        Your task is to {task}

        Follow these rules strictly:
        {CONVENTIONAL_COMMIT_RULES}

        Output format — {output_format}

        Now generate based on the following changes:

        {diff}"""

    response = client.chat.completions.create(
        model="gpt-4o-mini",
        messages=[
            {
                "role": "system",
                "content": "You generate high quality git commit messages.",
            },
            {"role": "user", "content": prompt},
        ],
        temperature=temperature,
    )
    raw: str = response.choices[0].message.content.replace("```", "").strip()
    return [m.strip() for m in raw.split("---") if m.strip()]


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
        click.echo(f"\n  ✗  Commit failed:\n\n{result.stderr.strip()}\n")
        return

    # Display Git's output on success
    click.echo(f"\n  ✓  Commit created successfully!\n")
    click.echo(result.stdout.strip())
    click.echo()


# Main command group entry — runs default behaviour when called with no subcommand
@click.group(invoke_without_command=True)
@click.pass_context
def cli(ctx: click.Context) -> None:
    if ctx.invoked_subcommand is None:
        main()


# Default behaviour: generates and applies a single commit message for staged changes
def main() -> None:
    diff: str | None = get_diff()
    if diff is None:
        return

    messages: list[str] | None = generate_commit_messages(diff)
    if messages is None:
        click.echo("\n  ✗  OpenAI API key not set.\n")
        return

    click.echo("\n  Proposed commit message:\n")
    click.echo(f"    {messages[0]}\n")

    if not click.confirm("  Accept and commit with this message?"):
        click.echo()
        return

    create_commit(messages[0])


# List command: generates N commit message suggestions and lets the user pick one
@cli.command("list")
@click.option(
    "-n",
    "--num",
    default=5,
    show_default=True,
    help="Number of commit message suggestions to generate.",
)
def list_commits(num: int) -> None:
    """
    Generate multiple commit message suggestions and commit with the chosen one.

    Generates NUM distinct Conventional Commit message suggestions based on
    the currently staged diff, presents them as an arrow-key navigable list,
    and creates the commit once the user confirms their selection.

    Args:
        num (int):
            Number of suggestions to generate (default: 5).
    """
    diff: str | None = get_diff()
    if diff is None:
        return

    click.echo(f"\n  Generating {num} commit message suggestions…\n")

    messages: list[str] | None = generate_commit_messages(diff, num)
    if messages is None:
        click.echo("\n  ✗  OpenAI API key not set.\n")
        return
    if not messages:
        click.echo("\n  ✗  No suggestions were returned. Please try again.\n")
        return

    # Add a blank line of padding above the selector
    click.echo()

    # Build the display choices for the interactive selector
    choices: list[questionary.Choice] = [
        questionary.Choice(title=f"  {message}", value=message) for message in messages
    ]

    selected: str | None = questionary.select(
        "  Select a commit message:",
        choices=choices,
        style=SELECTOR_STYLE,
        qmark="",
    ).ask()

    # ask() returns None if the user pressed Ctrl-C
    if selected is None:
        click.echo("\n  Aborted.\n")
        return

    click.echo(f"\n  Selected message:\n\n    {selected}\n")

    if not click.confirm("  Commit with this message?"):
        click.echo()
        return

    create_commit(selected)


# Program main entry
if __name__ == "__main__":
    cli()
