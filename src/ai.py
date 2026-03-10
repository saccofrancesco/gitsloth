# Standard library imports
import os

# Third-party imports
import openai


# Prompt template used to instruct the LLM to generate Conventional Commit messages
COMMIT_PROMPT_TEMPLATE: str = """
You are an expert software engineer that writes precise commit messages
following the Conventional Commits specification.

Generate {n} different commit messages for the following changes.

Rules:

1. Use the Conventional Commits format:
   <type>(optional scope): <short summary>

2. Allowed types:
   feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert

3. The summary must:
   - Be lowercase
   - Use imperative mood (e.g., "add", "fix")
   - Not end with a period
   - Be concise (max 72 characters)

4. If the change is breaking:
   - Add an exclamation mark after the type/scope (e.g., feat!:)
   - Add a "BREAKING CHANGE:" footer

5. Include a body separated by a blank line if additional context is needed.

Return ONLY the commit messages as a numbered list.

Changes:
{diff}
"""


def estimate_token_count(text: str) -> int:
    """
    Roughly estimate the number of tokens in a string.

    This heuristic assumes ~4 characters per token, which is a
    commonly used approximation for English text.

    Args:
        text (str):
            The input text.

    Returns:
        int:
            Estimated token count.
    """

    # Rough approximation: 4 characters ≈ 1 token
    return len(text) // 4


def generate_commit_messages(diff: str, n: int) -> list[str]:
    """
    Generate Conventional Commit messages based on a Git diff.

    The function sends the staged diff to an LLM and asks it to
    generate multiple commit message suggestions following the
    Conventional Commits specification.

    Args:
        diff (str):
            The staged Git diff.

        n (int):
            Number of commit suggestions to generate.

    Returns:
        list[str]:
            A list containing up to `n` commit message suggestions.

    Raises:
        EnvironmentError:
            If the OPENAI_API_KEY environment variable is not set.
    """

    # Retrieve the API key from environment variables
    api_key: str | None = os.getenv("OPENAI_API_KEY")

    if not api_key:
        raise EnvironmentError("OPENAI_API_KEY environment variable is not set.")

    # Initialize the OpenAI client
    client: openai.OpenAI = openai.OpenAI(api_key=api_key)

    # Format the prompt with the staged diff and requested number of commits
    prompt: str = COMMIT_PROMPT_TEMPLATE.format(diff=diff, n=n)

    # Send request to the model
    response = client.chat.completions.create(
        model="gpt-4o-mini",
        temperature=0.2,
        messages=[
            {
                "role": "system",
                "content": "You generate high quality git commit messages.",
            },
            {
                "role": "user",
                "content": prompt,
            },
        ],
    )

    # Extract response content
    response_text: str = response.choices[0].message.content.replace("```", "").strip()

    commits: list[str] = list()

    # Parse numbered list output from the model
    for line in response_text.splitlines():
        line: str = line.strip()

        if not line:
            continue

        # Remove numbering like "1. ", "2. "
        if "." in line:
            parts: list[str] = line.split(".", 1)
            if parts[0].isdigit():
                line: str = parts[1].strip()

        commits.append(line)

    return commits
