// Command gitsloth generates Conventional Commit messages from staged Git
// changes using the OpenAI API. It can generate one or more suggestions,
// allow user selection, optionally confirm, and either create the commit
// or copy it to the clipboard.
//
// Usage:
//
//	gitsloth                     Generate one message (with confirmation)
//	gitsloth -g 3               Generate multiple messages and select one
//	gitsloth [-a | --all]       Stage all changes before generating
//	gitsloth [-c | --clipboard] Copy message instead of committing
//
// Requirements:
//   - Must be executed inside a Git repository
//   - OPENAI_API_KEY environment variable must be set
package main

import (
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

func main() {
	var all bool
	var clipboard bool
	var generate int

	flag.BoolVar(&all, "all", false, "stage all changes before committing")
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

	ctx, err := buildGitContext()
	if err != nil {
		fmt.Println("Failed to build git context:", err)
		os.Exit(1)
	}

	if strings.TrimSpace(ctx.Diff) == "" {
		fmt.Println("No changes to commit")
		os.Exit(0)
	}

	messages, err := generateCommitMessages(*ctx, generate)
	if err != nil {
		fmt.Println("Failed to generate commit messages:", err)
		os.Exit(1)
	}

	message, ok := selectMessage(messages)
	if !ok {
		fmt.Println("Operation aborted")
		os.Exit(0)
	}

	if generate == 1 && !askForConfirmation(message) {
		fmt.Println("Operation aborted")
		os.Exit(0)
	}

	if clipboard {
		if err := copyToClipBoard(message); err != nil {
			fmt.Println("Failed to copy to clipboard:", err)
			os.Exit(1)
		}
		fmt.Println("Message copied to clipboard")
		return
	}

	if err := createCommit(message); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// isGitRepoHere checks if the current directory contains a .git folder.
func isGitRepoHere() bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(cwd, ".git"))
	return err == nil && info != nil
}

// stageAllChanges runs `git add -A`.
func stageAllChanges() error {
	cmd := exec.Command("git", "add", "-A")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add failed: %s", out)
	}
	return nil
}

// isCommandAvailable reports whether a command exists in PATH.
func isCommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// copyToClipBoard copies text to the system clipboard using an available tool.
func copyToClipBoard(text string) error {
	var cmd *exec.Cmd

	switch {
	case isCommandAvailable("pbcopy"):
		cmd = exec.Command("pbcopy")
	case isCommandAvailable("xclip"):
		cmd = exec.Command("xclip", "-selection", "clipboard")
	case isCommandAvailable("wl-copy"):
		cmd = exec.Command("wl-copy")
	case isCommandAvailable("clip"):
		cmd = exec.Command("cmd", "/c", "clip")
	default:
		return fmt.Errorf("no clipboard utility found")
	}

	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if _, err := io.WriteString(in, text); err != nil {
		return err
	}
	in.Close()

	return cmd.Wait()
}

func getBranchName() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	return strings.TrimSpace(string(out)), err
}

func getShortGitStatus() (string, error) {
	out, err := exec.Command("git", "status", "--short").Output()
	return strings.TrimSpace(string(out)), err
}

func getTruncatedDiff(max int) (string, error) {
	out, err := exec.Command("git", "diff", "--cached").Output()
	if err != nil {
		return "", err
	}
	diff := string(out)
	if len(diff) > max {
		diff = diff[:max] + "\n... (truncated)"
	}
	return diff, nil
}

// GitContext groups repository state used for prompt generation.
type GitContext struct {
	Branch string
	Status string
	Diff   string
}

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
	return &GitContext{branch, status, diff}, nil
}

// startSpinner displays a terminal spinner and returns a stop function.
func startSpinner(msg string) func() {
	chars := []rune("⣷⣯⣟⡿⢿⣻⣽⣾")
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		for i := 0; ; i++ {
			select {
			case <-stop:
				fmt.Print("\r\033[K")
				return
			default:
				fmt.Printf("\r%c %s", chars[i%len(chars)], msg)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	return func() {
		close(stop)
		<-done
	}
}

const ConventionalCommitRules = `
Use Conventional Commits:

<type>(optional scope): <summary>

Types:
feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert

Summary rules:
- lowercase
- no trailing period
- max 72 chars
- imperative mood
`

func generateCommitMessages(ctx GitContext, n int) ([]string, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}

	stop := startSpinner("Generating commit messages...")

	prompt := fmt.Sprintf(`
%s

Branch:
%s

Status:
%s

Diff:
%s

Generate %d commit messages as JSON array of strings.`,
		ConventionalCommitRules,
		ctx.Branch,
		ctx.Status,
		ctx.Diff,
		n,
	)

	body, _ := json.Marshal(map[string]any{
		"model": "gpt-4o-mini",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})

	req, _ := http.NewRequest("POST",
		"https://api.openai.com/v1/chat/completions",
		bytes.NewBuffer(body),
	)

	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		stop()
		return nil, err
	}
	defer resp.Body.Close()

	stop()

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string
			}
		}
	}

	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	content = strings.ReplaceAll(content, "```json", "")
	content = strings.ReplaceAll(content, "```", "")

	var msgs []string
	if err := json.Unmarshal([]byte(content), &msgs); err != nil {
		return nil, err
	}

	return msgs, nil
}

func selectMessage(msgs []string) (string, bool) {
	if len(msgs) == 1 {
		return msgs[0], true
	}

	fmt.Println("Generated messages:")
	for i, m := range msgs {
		fmt.Printf("%d) %s\n", i+1, m)
	}
	fmt.Print("Select (0 to abort): ")

	var choice int
	fmt.Scanf("%d", &choice)

	if choice <= 0 || choice > len(msgs) {
		return "", false
	}
	return msgs[choice-1], true
}

func askForConfirmation(msg string) bool {
	fmt.Println("Proposed message:\n", msg)
	fmt.Print("Confirm? (y/n): ")

	var input string
	fmt.Scanln(&input)
	input = strings.ToLower(strings.TrimSpace(input))
	return input == "y" || input == "yes"
}

func createCommit(msg string) error {
	cmd := exec.Command("git", "commit", "-m", msg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("commit failed: %s", out)
	}
	fmt.Println(string(out))
	return nil
}
