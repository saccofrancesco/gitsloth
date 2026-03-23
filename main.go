package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	} else {
		fmt.Println("Git diff:")
		fmt.Println(diff)
	}
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
