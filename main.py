# Importing libraries
import os
import openai
import subprocess
import sys


# Based on a given git diff string, automatically generates commit messages which
# follow the Conventional Commit's rules
def generate_commit_message(diff: str) -> str:
    api_key: str = os.getenv("OPENAI_API_KEY")
    if not api_key:
        print("OpenAI Api Key no setted...")
        sys.exit(1)
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
        model="gpt-4o-mini",  # cost-efficient and good quality
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
        stdout=subprocess.PIPE,  # Capture standard output (the diff content)
        stderr=subprocess.PIPE,  # Capture error output (if any)
        text=True,  # Return output as a string instead of bytes
    )

    # Remove leading/trailing whitespace for cleaner downstream usage
    return result.stdout.strip()


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
    diff: subprocess.CompletedProcess = get_staged_diff()
    if not diff:
        print("No staged changes found...")
        sys.exit(0)
    message: str = generate_commit_message(diff)
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
