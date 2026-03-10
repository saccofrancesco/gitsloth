"""
Custom exceptions used throughout the Gitsloth CLI application.

These exceptions provide a clear error hierarchy so that
application-level errors can be caught and handled gracefully
without masking unexpected Python exceptions.
"""

__all__: list[str] = [
    "GitslothError",
    "NotARepositoryError",
    "NoStagedChangesError",
    "TokenLimitExceededError",
]


class GitslothError(Exception):
    """
    Base exception for all Gitsloth-specific errors.

    This allows the CLI to catch application errors separately
    from unexpected runtime errors.
    """


class NotARepositoryError(GitslothError):
    """
    Raised when the current directory is not inside a Git repository.

    Typically triggered when the tool is executed outside a folder
    initialized with `git init` or not within a Git project.
    """


class NoStagedChangesError(GitslothError):
    """
    Raised when there are no staged changes available to commit.

    This occurs when `git add` has not been run or no files have
    been modified.
    """


class TokenLimitExceededError(GitslothError):
    """
    Raised when the staged Git diff exceeds the allowed token estimate.

    This protects the application from sending excessively large
    payloads to the LLM API which could exceed context limits
    or generate unnecessary API costs.
    """
