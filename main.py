# Importing libraries
import click
import subprocess
import os
import openai


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


# Helper function called if inside a .git repo that search for staged files
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


# Based on a given git diff string, automatically generates commit messages which
# follow the Conventional Commit's rules
def generate_commit_message(diff: str) -> str | None:
    """
    Generate Conventional Commit message based on a Git diff.

    The function sends the staged diff to an LLM and asks it to
    generate multiple commit message suggestions following the
    Conventional Commits specification.

    Args:
        diff (str):
            The staged Git diff.

    Returns:
        str:
            A string containing the suggested commit message.
    """
    api_key: str = os.getenv("OPENAI_API_KEY")
    if not api_key:
        return
    client: openai.OpenAI = openai.OpenAI(api_key=api_key)
    prompt: str = f"""
        You are an expert software engineer that writes precise commit messages following the Conventional Commits specification.

        Your task is to generate a properly formatted commit message based on the provided changes.

        Follow these rules strictly:

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

        5. If additional context is helpful, include a body separated by a blank line.

        Now generate a commit message based on the following changes:

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
        temperature=0.2,
    )
    message: str = response.choices[0].message.content.replace("```", "").strip()
    return message


# Helper function which given a text as commit messages does the commit process
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
        click.echo(f"Commit failed:\n{result.stderr.strip()}")

    # Display Git's output on success
    click.echo("\nCommit created successfully!")
    click.echo(result.stdout.strip())


# Main command line entry
@click.command()
def main() -> None:
    if not is_git_repo():
        click.echo("Not inside a .git repository.")
        return
    diff: str = get_staged_diff()
    if not diff:
        click.echo("No staged diff founded.")
        return
    message: str = generate_commit_message(diff)
    if message is None:
        click.echo("OpenAI Api Key no setted.")
        return
    click.echo("Proposed commit message:\n")
    click.echo(f"\t{message}\n")
    if not click.confirm("\nAccept and commit with this message?"):
        return
    create_commit(message)


# Program main entry
if __name__ == "__main__":
    main()
