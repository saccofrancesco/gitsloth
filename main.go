// Command gitsloth generates a Conventional Commit message from staged Git changes
// using the OpenAI API, asks for user confirmation, and creates the commit.
//
// Usage:
//
//		gitsloth                        Generate one commit message on the staged changes
//		gitsloth [-a | --all]           Stage all the changes before generating the commit
//	    gitsloth [-c | --clipboard]     Copy the generated message instead of committing
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
	var clipboard bool
	var generate int
	flag.BoolVar(&all, "all", false, "stage all changes before commiting")
	flag.BoolVar(&all, "a", false, "shorthand for --all")
	flag.BoolVar(&clipboard, "clipboard", false, "copy selected message to clipboard")
	flag.BoolVar(&clipboard, "c", false, "shorthand for --clipboard")
	flag.IntVar(&generate, "generate", 1, "number of commit messages to generate")
	flag.IntVar(&generate, "g", 1, "shorthand for --generate")
	flag.Parse()

	if generate < 1 {
		fmt.Println("generate must be >= 1")
		os.Exit(1)
	}

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
	ctx, err := buildGitContext()
	if err != nil {
		fmt.Println("Failed to build git context:", err)
		os.Exit(1)
	}

	// Ensure there are actual staged changes before proceeding.
	if strings.TrimSpace(ctx.Diff) == "" {
		fmt.Println("No changes to commit")
		os.Exit(0)
	}

	messages, err := generateCommitMessages(*ctx, generate)
	if err != nil {
		fmt.Println("Failed to generate commit messages", err)
		os.Exit(1)
	}
	message, ok := selectMessage(messages)
	if !ok {
		fmt.Println("Operation aborted")
		os.Exit(0)
	}

	if !askForConfirmation(message) {
		fmt.Println("Operation aborted")
		os.Exit(0)
	}

	if clipboard {
		if err := copyToClipBoard(message); err != nil {
			fmt.Println("Failed to copy to clipboard:", err)
			os.Exit(1)
		}
		fmt.Println("Message copied to clipboard")
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
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	gitPath := filepath.Join(cwd, ".git")
	info, err := os.Stat(gitPath)
	return err == nil && info != nil
}

// stageAllChanges stages all changes in the repository by running `git add -A`.
// It returns an error if the Git command fails, including the command output
// for easier debugging.
func stageAllChanges() error {
	cmd := exec.Command("git", "add", "-A")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add failed: %s", string(output))
	}
	return nil
}

// isCommandAvailable checks whether a given executable is present
// in the system PATH. It is used to detect which clipboard utility
// can be invoked on the current machine.
func isCommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// copyToClipboard copies the provided text to the system clipboard.
// It detects the available clipboard utility depending on the OS:
// - pbcopy (macOS)
// - xclip or wl-copy (Linux)
// - clip (Windows)
// Returns an error if no supported clipboard command is found
// or if the execution fails.
func copyToClipBoard(text string) error {
	var cmd *exec.Cmd
	switch {
	case isCommandAvailable("pbcopy"): // macOS
		cmd = exec.Command("pbcopy")
	case isCommandAvailable("xclip"): // Linux (X11)
		cmd = exec.Command("xclip", "-selection", "clipboard")
	case isCommandAvailable("wl-copy"): // Linux (Wayland)
		cmd = exec.Command("wl-copy")
	case isCommandAvailable("clip"): // Windows
		cmd = exec.Command("cmd", "/c", "clip")
	default:
		return fmt.Errorf("No clipboard utility found (pbcopy, xclip, wl-copy, clip)")
	}

	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	_, err = io.WriteString(in, text)
	if err != nil {
		return err
	}
	in.Close()

	return cmd.Wait()
}

// getBranchName returns the current Git branch name.
func getBranchName() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getShortGitStatus returns a compact representation of staged/unstaged changes.
func getShortGitStatus() (string, error) {
	cmd := exec.Command("git", "status", "--short")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getTruncatedDiff returns the staged Git diff (git diff --cached),
// truncated to avoid sending excessively large payloads to the API.
func getTruncatedDiff(maxBytes int) (string, error) {
	cmd := exec.Command("git", "diff", "--cached")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	diff := string(output)
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
	branch, err := getBranchName()
	if err != nil {
		return nil, err
	}

	status, err := getShortGitStatus()
	if err != nil {
		return nil, err
	}

	diff, err := getTruncatedDiff(8000)
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
	chars := []rune("⣷⣯⣟⡿⢿⣻⣽⣾")
	stop := make(chan struct{})
	done := make(chan struct{})

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
const ConventionalCommitRules = `
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
func generateCommitMessages(ctx GitContext, n int) ([]string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}
	stop := startSpinner(" Generating commit messages...")
	prompt := fmt.Sprintf(`
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
Generate %d different commit messages.

Return ONLY a valid JSON array of strings.
Example:
["feat: add login endpoint", "fix: handle nil pointer"]
`,
		ConventionalCommitRules,
		ctx.Branch,
		ctx.Status,
		ctx.Diff,
		n,
	)

	body := map[string]any{
		"model": "gpt-4o-mini",
		"messages": []map[string]string{
			{"role": "system", "content": "You write excellent commit messages."},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.5,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		stop()
		return nil, err
	}

	req, err := http.NewRequest(
		"POST",
		"https://api.openai.com/v1/chat/completions",
		bytes.NewBuffer(jsonBody),
	)
	if err != nil {
		stop()
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		stop()
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		stop()
		return nil, fmt.Errorf("API error: %s", string(respBody))
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
		return nil, err
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no response choices returned")
	}

	content := result.Choices[0].Message.Content
	content = strings.ReplaceAll(content, "```json", "")
	content = strings.ReplaceAll(content, "```", "")
	content = strings.TrimSpace(content)

	var messages []string
	if err := json.Unmarshal([]byte(content), &messages); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v\nraw: %s", err, content)
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages generated")
	}

	return messages, nil
}

func selectMessage(messages []string) (string, bool) {
	if len(messages) == 1 {
		return messages[0], true
	}

	fmt.Println("Generated commit messages:")
	for i, msg := range messages {
		fmt.Printf("%d) %s\n", i+1, msg)
	}
	fmt.Print("Select a message (number) or 0 to abort: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	var choice int
	fmt.Sscanf(input, "%d", &choice)
	if choice <= 0 || choice > len(messages) {
		return "", false
	}

	return messages[choice-1], true
}

// askForConfirmation displays the proposed commit message and asks the user
// for confirmation via standard input. It returns true if the user accepts.
func askForConfirmation(message string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Proposed commit message:")
	fmt.Println(message)
	fmt.Print("Accept and commit? (y/n): ")

	input, err := reader.ReadString('\n')
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
	cmd := exec.Command("git", "commit", "-m", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("commit failed: %s", string(output))
	}

	fmt.Println("Commit created succesfully")
	fmt.Println(string(output))
	return nil
}
