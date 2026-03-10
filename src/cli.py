# Importing standard libraries
import sys
import argparse

# Importing third-party libraries for CLI UX
from halo import Halo
import questionary

# Importing project modules for git interaction, AI generation and UI
from src.git import is_git_repository, get_staged_diff, create_commit
from src.ai import generate_commit_messages, estimate_token_count
from src.ui import choose_commit
from src.exceptions import (
    NotARepositoryError,
    NoStagedChangesError,
    TokenLimitExceededError,
)

# Maximum allowed token estimate for the diff sent to the LLM
# This protects against excessively large diffs that would exceed
# the model's context window or unnecessarily increase API cost.
MAX_TOKEN_ESTIMATE: int = 6_000


def parse_args() -> argparse.Namespace:
    """
    Parse CLI arguments for the gitsloth command-line interface.

    This function defines the available CLI commands and options.
    Currently supported:

    - list: generate multiple commit suggestions

    Returns:
        argparse.Namespace:
            Parsed command-line arguments containing user options.
    """
    # Creating the root CLI parser
    parser = argparse.ArgumentParser(
        description="Generate AI-powered Git commit messages."
    )

    # Adding support for subcommands
    subparsers: argparse._SubParsersAction = parser.add_subparsers(dest="command")

    # Subcommand: list
    # Generates multiple commit suggestions instead of a single one
    list_parser: argparse.ArgumentParser = subparsers.add_parser(
        "list", help="Generate multiple commit suggestions"
    )

    # Optional argument: number of suggestions to generate
    list_parser.add_argument(
        "-n",
        "--num",
        type=int,
        default=5,
        help="Number of commit suggestions to generate",
    )

    # Parse CLI input and return the namespace
    return parser.parse_args()


def main() -> None:
    """
    Main CLI entry point for the GitSloth commit generator.

    Workflow:
        1. Ensure the current directory is a valid Git repository.
        2. Retrieve the staged Git diff.
        3. Estimate token usage to prevent excessive API requests.
        4. Generate commit suggestions using the OpenAI API.
        5. Allow the user to select or confirm a commit message.
        6. Create the Git commit with the selected message.

    Raises:
        NotARepositoryError:
            If the command is executed outside of a Git repository.

        NoStagedChangesError:
            If no files are staged for commit.

        TokenLimitExceededError:
            If the staged diff is too large for the configured limit.
    """
    try:

        # Parse CLI arguments
        args: argparse.Namespace = parse_args()

        # Ensure the command is executed inside a Git repository
        if not is_git_repository():
            raise NotARepositoryError("Not inside a Git repository.")

        # Retrieve the staged changes diff
        diff: str = get_staged_diff()

        # Prevent generating commits when nothing is staged
        if not diff:
            raise NoStagedChangesError("No staged changes found.")

        # Protect the LLM request from excessively large diffs
        if estimate_token_count(diff) > MAX_TOKEN_ESTIMATE:
            raise TokenLimitExceededError("Diff exceeds the maximum token estimate.")

        # Determine how many suggestions should be generated
        # Default behaviour -> single commit
        # list command -> multiple suggestions
        n: int = args.num if args.command == "list" else 1

        # Display a loading spinner while generating AI suggestions
        with Halo(text="Generating commit messages...", spinner="dots"):
            commits: list[str] = generate_commit_messages(diff, n)

        # If the user requested multiple suggestions,
        # show an interactive selector
        if args.command == "list":
            message: str = choose_commit(commits)
        else:
            message: str = commits[0]
            print(message)

        # Ask user confirmation before creating the commit
        confirm: bool = questionary.confirm("Commit with this message?", qmark="").ask()

        if not confirm:
            print("Commit aborted.")
            sys.exit(0)

        # Create the Git commit using the selected message
        create_commit(message)

    # Handle expected CLI errors gracefully
    except (NotARepositoryError, NoStagedChangesError, TokenLimitExceededError) as e:
        print(f"Error: {e}")
        sys.exit(1)
