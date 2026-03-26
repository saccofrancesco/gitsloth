// Command gitsloth generates a Conventional Commit message from staged Git changes
// using the OpenAI API, asks for user confirmation, and creates the commit.
//
// It requires:
//   - Being inside a Git repository
//   - OPENAI_API_KEY environment variable set
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// main is the entry point of the CLI tool. It validates the environment,
// generates a commit message from the staged diff, asks for confirmation,
// and creates the commit.
func main() {
	var all bool
	flag.BoolVar(&all, "all", false, "stage all changes before commiting")
	flag.BoolVar(&all, "a", false, "stage all changes before commiting (shorthand)")
	flag.Parse()

	if !isGitRepoHere() {
		fmt.Println("Not inside a Git repository (.git not found here)")
		os.Exit(1)
	}

	if all {
		if err := stageAllChanges(); err != nil {
			fmt.Println("Failed to stage changes:", err)
			os.Exit(1)
		}
	}

	// Build structured Git context instead of relying on raw diff only.
	var ctx *GitContext
	var err error
	ctx, err = buildGitContext()
	if err != nil {
		fmt.Println("Failed to build git context:", err)
		os.Exit(1)
	}

	// Ensure there are actual staged changes before proceeding.
	if strings.TrimSpace(ctx.Diff) == "" {
		fmt.Println("No changes to commit")
		os.Exit(0)
	}

	var message string
	message, err = generateCommitMessage(*ctx)
	if err != nil {
		fmt.Println("Failed to generate the commit message", err)
		os.Exit(1)
	} else if message == "" {
		fmt.Println("Commit message is empty")
		os.Exit(1)
	}

	if !askForConfirmation(message) {
		fmt.Println("Commit aborted")
		os.Exit(0)
	}

	err = createCommit(message)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// isGitRepoHere reports whether the current working directory
// contains a .git folder, indicating it is inside a Git repository.
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

// stageAllChanges stages all changes in the repository by running `git add -A`.
// It returns an error if the Git command fails, including the command output
// for easier debugging.
func stageAllChanges() error {
	var cmd *exec.Cmd = exec.Command("git", "add", "-A")
	var output []byte
	var err error
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add failed: %s", string(output))
	}
	return nil
}

// getBranchName returns the current Git branch name.
func getBranchName() (string, error) {
	var cmd *exec.Cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	var output []byte
	var err error
	output, err = cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getShortGitStatus returns a compact representation of staged/unstaged changes.
func getShortGitStatus() (string, error) {
	var cmd *exec.Cmd = exec.Command("git", "status", "--short")
	var output []byte
	var err error
	output, err = cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getTruncatedDiff returns the staged Git diff (git diff --cached),
// truncated to avoid sending excessively large payloads to the API.
func getTruncatedDiff(maxBytes int) (string, error) {
	var cmd *exec.Cmd = exec.Command("git", "diff", "--cached")
	var output []byte
	var err error
	output, err = cmd.Output()
	if err != nil {
		return "", err
	}

	var diff string = string(output)
	if len(diff) > maxBytes {
		diff = diff[:maxBytes] + "\n... (truncated)"
	}
	return diff, nil
}

// GitContext contains structured information about the current Git state.
// This improves LLM output quality compared to using only raw diffs.
type GitContext struct {
	Branch string
	Status string
	Diff   string
}

// buildGitContext gathers all relevant Git information into a single struct.
func buildGitContext() (*GitContext, error) {
	var branch string
	var err error
	branch, err = getBranchName()
	if err != nil {
		return nil, err
	}

	var status string
	status, err = getShortGitStatus()
	if err != nil {
		return nil, err
	}

	var diff string
	diff, err = getTruncatedDiff(8000)
	if err != nil {
		return nil, err
	}

	return &GitContext{
		Branch: branch,
		Status: status,
		Diff:   diff,
	}, nil
}

// startSpinner displays a terminal spinner with the provided message.
// It runs in a separate goroutine and returns a stop function that
// blocks until the spinner has fully stopped and the line is cleared.
func startSpinner(message string) func() {
	var chars []rune = []rune("⣾⣽⣻⢿⡿⣟⣯⣷")
	var stop chan struct{} = make(chan struct{})
	var done chan struct{} = make(chan struct{})

	go func() {
		defer close(done)
		var i int = 0
		for {
			select {
			case <-stop:
				fmt.Print("\r\033[K")
				return
			default:
				fmt.Printf("\r%c %s", chars[i%len(chars)], message)
				time.Sleep(100 * time.Millisecond)
				i++
			}
		}
	}()

	return func() {
		close(stop)
		<-done
	}
}

// ConventionalCommitRules defines the formatting rules used to guide
// the language model when generating commit messages.
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

// generateCommitMessage uses the OpenAI HTTP API to generate a
// Conventional Commit message based on the provided Git context.
//
// It starts a spinner while the request is in progress and ensures
// the spinner is stopped before returning.
//
// Requirements:
//   - OPENAI_API_KEY environment variable must be set
//
// The returned message is cleaned of formatting artifacts (e.g., code fences).
func generateCommitMessage(ctx GitContext) (string, error) {
	var apiKey string = os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY not set")
	}

	var stop func() = startSpinner(" Generating commit message...")

	// Build a structured prompt using multiple signals instead of raw diff only.
	var prompt string = fmt.Sprintf(`
You are an expert software engineer that writes precise commit messages.

Follow the Conventional Commits specification.

%s

Branch:
%s

Git status:
%s

Diff:
%s

Task:
Generate ONE properly formatted commit message.
Return ONLY the commit message.
`,
		ConventionalCommitRules,
		ctx.Branch,
		ctx.Status,
		ctx.Diff,
	)

	var body map[string]interface{} = map[string]interface{}{
		"model": "gpt-4o-mini",
		"messages": []map[string]string{
			{"role": "system", "content": "You write excellent commit messages."},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.2,
	}

	var jsonBody []byte
	var err error
	jsonBody, err = json.Marshal(body)
	if err != nil {
		stop()
		return "", err
	}

	var req *http.Request
	req, err = http.NewRequest(
		"POST",
		"https://api.openai.com/v1/chat/completions",
		bytes.NewBuffer(jsonBody),
	)
	if err != nil {
		stop()
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	var client *http.Client = &http.Client{}
	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		stop()
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var respBody []byte
		respBody, _ = io.ReadAll(resp.Body)
		stop()
		return "", fmt.Errorf("API error: %s", string(respBody))
	}

	stop()

	type chatCompletionResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	var result chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	var message string = result.Choices[0].Message.Content
	message = strings.ReplaceAll(message, "```", "")
	message = strings.TrimSpace(message)

	return message, nil
}

// askForConfirmation displays the proposed commit message and asks the user
// for confirmation via standard input. It returns true if the user accepts.
func askForConfirmation(message string) bool {
	var reader *bufio.Reader = bufio.NewReader(os.Stdin)
	fmt.Println("Proposed commit message:")
	fmt.Println(message)
	fmt.Print("Accept and commit? (y/n): ")

	var input string
	var err error
	input, err = reader.ReadString('\n')
	if err != nil {
		return false
	}

	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

// createCommit creates a Git commit using the provided commit message.
// It executes `git commit -m <message>` and returns an error if the
// command fails. On success, it prints the Git output.
func createCommit(message string) error {
	var cmd *exec.Cmd = exec.Command("git", "commit", "-m", message)
	var output []byte
	var err error
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("commit failed: %s", string(output))
	}

	fmt.Println("Commit created succesfully")
	fmt.Println(string(output))
	return nil
}
