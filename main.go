package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
    fmt.Println("Hello Gitsloth in Go!")
}

func isGitRepoHere() bool {
    cwd, err := os.Getwd()
    if err != nil {
        return false
    }
    gitPath := filepath.Join(cwd, ".git")
    info, err := os.Stat(gitPath)
    return err == nil && info != nil
}