package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	openai "github.com/openai/openai-go"
)

func main() {
	if !isGitRepoHere() {
		fmt.Println("Not inside a Git repository (.git not found here)")
		os.Exit(1)
	}
	diff, err := getGitDiff()
	if err != nil {
		fmt.Println("Failed to get `git diff`", err)
		os.Exit(1)
	}
	if diff == "" {
		fmt.Println("No changes to commit")
		os.Exit(0)
	}
	message, err := generateCommitMessage(diff)
	if err != nil {
		fmt.Println("Failed to generate the commit message", err)
		os.Exit(1)
	} else if message == "" {
		fmt.Println("Commit message is empty")
		os.Exit(1)
	}
	confirmation := askForConfirmation(message)
	if !confirmation {
		fmt.Println("Commit aborted")
		os.Exit(1)
	}
	createCommit(message)
}

func isGitRepoHere() bool {
	var cwd string
	var err error
	cwd, err = os.Getwd()
	if err != nil {
		return false
	}
	var gitPath string = filepath.Join(cwd, ".git")
	var info os.FileInfo
	info, err = os.Stat(gitPath)
	return err == nil && info != nil
}

func getGitDiff() (string, error) {
	cmd := exec.Command("git", "diff", "--cached")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

const ConventionalCommitRules string = `
1. Use the Conventional Commits format:
<type>(optional scope): <short summary>

2. Allowed types:
feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert

3. The summary must:
- Be in lowercase
- Not end with a period
- Be concise (max 72 characters)
- Use imperative mood (e.g., "add", "fix", not "added", "fixes")
`

func generateCommitMessage(diff string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY not set")
	}
	client := openai.NewClient()
	prompt := fmt.Sprintf(`
	You are an expert software engineer that writes precise commit messages.

	Follow the Conventional Commits specification.

	%s

	Task:
	Generate ONE properly formatted commit message for the following git diff.

	Changes:
	%s

	Return ONLY the commit message.
	`, ConventionalCommitRules, diff)
	resp, err := client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model: "gpt-4o-mini",
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You write excelent commit messages."),
			openai.UserMessage(prompt),
		},
		Temperature: openai.Float(0.2),
	})
	if err != nil {
		return "", err
	}
	message := resp.Choices[0].Message.Content
	message = strings.ReplaceAll(message, "```", "")
	message = strings.TrimSpace(message)
	return message, nil
}

func askForConfirmation(message string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Proposed commit message:")
	fmt.Println(message)
	fmt.Print("Accept and commit? (y/n): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

func createCommit(message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("commit failed: %s", string(output))
	}
	fmt.Println("Commit created succesfully")
	fmt.Println(string(output))
	return nil
}
