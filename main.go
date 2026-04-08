// Command gitsloth generates a Conventional Commit message from staged Git changes
// using the OpenAI API, asks for user confirmation, and creates the commit.
//
// Usage:
//
//	gitsloth [-a | --all]                      Generate one commit message and confirm
//	gitsloth list [-n | --num <count>]         Generate N commit messages and pick one
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
	"strconv"
	"strings"
	"time"
)

// main is the entry point of the CLI tool. It dispatches to either the
// default single-message flow or the list subcommand based on os.Args.
func main() {
	// Detect subcommand before flag parsing so that each subcommand
	// can own its own FlagSet without polluting the global one.
	if len(os.Args) > 1 && os.Args[1] == "list" {
		runList(os.Args[2:])
		return
	}

	runDefault(os.Args[1:])
}

// runDefault is the original single-message flow.
// Flags: -a / --all  (stage all changes before committing)
func runDefault(args []string) {
	fs := flag.NewFlagSet("gitsloth", flag.ExitOnError)
	var all bool
	fs.BoolVar(&all, "all", false, "stage all changes before committing")
	fs.BoolVar(&all, "a", false, "stage all changes before committing (shorthand)")
	fs.Parse(args) //nolint:errcheck // ExitOnError handles this

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

	ctx, err := buildGitContext()
	if err != nil {
		fmt.Println("Failed to build git context:", err)
		os.Exit(1)
	}

	if strings.TrimSpace(ctx.Diff) == "" {
		fmt.Println("No changes to commit")
		os.Exit(0)
	}

	messages, err := generateCommitMessages(*ctx, 1)
	if err != nil {
		fmt.Println("Failed to generate the commit message:", err)
		os.Exit(1)
	}
	if len(messages) == 0 || messages[0] == "" {
		fmt.Println("Commit message is empty")
		os.Exit(1)
	}

	message := messages[0]

	if !askForConfirmation(message) {
		fmt.Println("Commit aborted")
		os.Exit(0)
	}

	if err := createCommit(message); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// runList is the list subcommand: it generates N candidate commit messages,
// lets the user pick one interactively, then creates the commit.
// Flags: -n / --num  (number of messages to generate, default 5)
func runList(args []string) {
	fs := flag.NewFlagSet("gitsloth list", flag.ExitOnError)
	var num int
	fs.IntVar(&num, "num", 5, "number of commit messages to generate")
	fs.IntVar(&num, "n", 5, "number of commit messages to generate (shorthand)")
	fs.Parse(args) //nolint:errcheck // ExitOnError handles this

	if num < 1 {
		fmt.Println("--num must be at least 1")
		os.Exit(1)
	}

	if !isGitRepoHere() {
		fmt.Println("Not inside a Git repository (.git not found here)")
		os.Exit(1)
	}

	ctx, err := buildGitContext()
	if err != nil {
		fmt.Println("Failed to build git context:", err)
		os.Exit(1)
	}

	if strings.TrimSpace(ctx.Diff) == "" {
		fmt.Println("No changes to commit")
		os.Exit(0)
	}

	messages, err := generateCommitMessages(*ctx, num)
	if err != nil {
		fmt.Println("Failed to generate commit messages:", err)
		os.Exit(1)
	}
	if len(messages) == 0 {
		fmt.Println("No commit messages were generated")
		os.Exit(1)
	}

	chosen := chooseAnOption(messages)

	if err := createCommit(chosen); err != nil {
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
		i := 0
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

// generateCommitMessages uses the OpenAI HTTP API to generate one or more
// Conventional Commit messages based on the provided Git context.
//
// When count is 1 the model returns a plain string response.
// When count > 1 it uses JSON mode, asking the model to return a JSON object
// {"messages": ["...", "..."]} which is decoded directly — no text parsing needed.
//
// It starts a spinner while the request is in progress.
//
// Requirements:
//   - OPENAI_API_KEY environment variable must be set
func generateCommitMessages(ctx GitContext, count int) ([]string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}

	spinnerMsg := " Generating commit message..."
	if count > 1 {
		spinnerMsg = fmt.Sprintf(" Generating %d commit messages...", count)
	}
	stop := startSpinner(spinnerMsg)

	var (
		systemPrompt string
		userPrompt   string
	)

	if count == 1 {
		systemPrompt = "You write excellent commit messages."
		userPrompt = fmt.Sprintf(`You are an expert software engineer that writes precise commit messages.

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
Return ONLY the commit message, with no preamble or explanation.
`, ConventionalCommitRules, ctx.Branch, ctx.Status, ctx.Diff)
	} else {
		// JSON mode: the system prompt must mention JSON so the model honours it.
		systemPrompt = `You write excellent commit messages. You always respond with valid JSON.`
		userPrompt = fmt.Sprintf(`You are an expert software engineer that writes precise commit messages.

Follow the Conventional Commits specification.

%s

Branch:
%s

Git status:
%s

Diff:
%s

Task:
Generate exactly %d distinct, properly formatted commit messages that explore different angles or phrasings of the same change.
Return a JSON object with a single key "messages" whose value is an array of exactly %d strings.
Example format: {"messages": ["feat: add foo", "feat(bar): introduce foo support", ...]}
`, ConventionalCommitRules, ctx.Branch, ctx.Status, ctx.Diff, count, count)
	}

	body := map[string]any{
		"model": "gpt-4o-mini",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.6,
	}
	if count > 1 {
		body["response_format"] = map[string]string{"type": "json_object"}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		stop()
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		stop()
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		stop()
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		stop()
		return nil, fmt.Errorf("API error: %s", string(body))
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

	content := strings.TrimSpace(result.Choices[0].Message.Content)

	// Single-message path: return the raw text as-is.
	if count == 1 {
		content = strings.ReplaceAll(content, "```", "")
		return []string{strings.TrimSpace(content)}, nil
	}

	// Multi-message path: decode the guaranteed JSON object.
	var payload struct {
		Messages []string `json:"messages"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return nil, fmt.Errorf("failed to decode JSON response: %w\nraw content: %s", err, content)
	}
	if len(payload.Messages) == 0 {
		return nil, fmt.Errorf("model returned an empty messages array")
	}

	return payload.Messages, nil
}

// chooseAnOption presents a numbered list of options and prompts the user to
// pick one by number. It loops until a valid choice is entered and returns
// the selected string.
func chooseAnOption(options []string) string {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("Proposed commit messages:")
		for i, opt := range options {
			fmt.Printf("  %d) %s\n", i+1, opt)
		}
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		choice, err := strconv.Atoi(input)
		if err != nil || choice < 1 || choice > len(options) {
			fmt.Printf("Invalid input — enter a number between 1 and %d.\n", len(options))
			continue
		}
		selected := options[choice-1]
		fmt.Println("Selected:", selected)
		return selected
	}
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

	fmt.Println("Commit created successfully")
	fmt.Println(string(output))
	return nil
}
