# Importing libraries
import subprocess
import sys


# Checks whether the current working directory is inside a Git repository
def is_git_repository() -> bool:
    """
    Returns True if the current directory is inside a Git working tree,
    otherwise returns False.
    """
    try:
        # Run a Git command that succeeds only if we are inside a repository.
        # If the command fails, subprocess will raise CalledProcessError
        # because check=True is set.
        subprocess.run(
            ["git", "rev-parse", "--is-inside-work-tree"],
            check=True,
            stdout=subprocess.PIPE,  # Suppress standard output
            stderr=subprocess.PIPE,  # Suppress error output
        )
        return True

    except subprocess.CalledProcessError:
        # The command failed, meaning we are not inside a Git repository
        return False


## Retrieve the diff of staged changes in the current Git repository
def get_staged_diff() -> str:
    """
    Returns the diff of staged (cached) changes.

    Executes `git diff --cached` to capture modifications that have been
    added to the staging area but not yet committed.
    """
    result: subprocess.CompletedProcess = subprocess.run(
        ["git", "diff", "--cached"],
        stdout=subprocess.PIPE,   # Capture standard output (the diff content)
        stderr=subprocess.PIPE,   # Capture error output (if any)
        text=True,                # Return output as a string instead of bytes
    )

    # Remove leading/trailing whitespace for cleaner downstream usage
    return result.stdout.strip()

# Fake commit message generator used for initial testing
def generate_commit_message() -> str:
    return "fix: create_commit function now properly handles commit messages and executes git commit command"

# Based on a given commit message, commits the changes to the detected repository
def create_commit(message: str) -> None:
    result: subprocess.CompletedProcess = subprocess.run(
        ["git", "commit", "-m", message],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    if result.returncode != 0:
        print("Commit failed...")
        print(result.stderr)
        sys.exit(1)
    else:
        print("Commit created successfully!")
        print(result.stdout)


# Main entry point of the application
def main() -> None:
    if not is_git_repository():
        print("Not inside a Git repository...")
        sys.exit(1)
    print("Git repository detected!")
    diff: subprocess.CompletedProcess = get_staged_diff()
    if not diff:
        print("No staged changes found...")
        sys.exit(0)
    print("Staged changes detected:")
    print(diff)
    message: str = generate_commit_message()
    print("Proposed Commit message:")
    print(message)
    confirm: str = input("Commit with this message? (y/n): ").strip().lower()
    if confirm != "y":
        print("Commit aborted...")
        sys.exit(0)
    create_commit(message)


# Execute the application only when the script is run directly,
# not when it is imported as a module.
if __name__ == "__main__":
    main()
