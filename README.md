<div align="center">
  <h1>gitsloth: AI-Powered Git Commit Messages</h1>
</div>

<p align="center">
  <a href="https://buymeacoffee.com/saccofrancesco">
    <img src="https://img.shields.io/badge/Buy%20Me%20a%20Coffee-ffdd00?style=for-the-badge&logo=buy-me-a-coffee&logoColor=black" />
  </a>
</p>

<h4 align="center">
A lightweight Rust CLI that generates clean, Conventional Commit messages from your staged Git changes using OpenAI.
</h4>

<p align="center">
  <img src="https://img.shields.io/github/contributors/saccofrancesco/gitsloth?style=for-the-badge" alt="Contributors">
  <img src="https://img.shields.io/github/forks/saccofrancesco/gitsloth?style=for-the-badge" alt="Forks">
  <img src="https://img.shields.io/github/stars/saccofrancesco/gitsloth?style=for-the-badge" alt="Stars">
</p>

<p align="center">
  <a href="#tldr">TL;DR</a> •
  <a href="#key-features">Key Features</a> •
  <a href="#quickstart">Quickstart</a> •
  <a href="#license">License</a>
</p>

---

## TL;DR

gitsloth reads your staged Git diff (plus branch name and status), asks OpenAI for one or more Conventional Commit messages, lets you select one, then commits it (or copies it to your clipboard).

---

## Why gitsloth

- Writing high-quality commit messages is repetitive
- Conventional Commits are useful but often tedious
- This tool automates the process while staying minimal

---

## Key Features

- AI-generated Conventional Commit messages
- Multiple suggestions with interactive selection
- Git context: branch, short status, staged diff
- Optional stage-all flow (`-a`)
- Optional clipboard flow (`-c`)
- Optional confirmation bypass for single message (`-y`)
- Minimal dependency strategy (Rust stdlib + only HTTP/JSON crates)

---

## Quickstart

### Install prebuilt binary (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/saccofrancesco/gitsloth/main/install.sh | sh
```

Installer behavior:
- detects macOS/Linux and CPU architecture automatically
- downloads the matching release asset from GitHub Releases
- installs to `/usr/local/bin` when writable, otherwise `~/.local/bin`
- adds the install directory to your shell profile when needed

Optional overrides:

```bash
# Install a specific tag
GITSLOTH_VERSION=v1.0.0 curl -fsSL https://raw.githubusercontent.com/saccofrancesco/gitsloth/main/install.sh | sh

# Install to a custom directory
GITSLOTH_INSTALL_DIR="$HOME/bin" curl -fsSL https://raw.githubusercontent.com/saccofrancesco/gitsloth/main/install.sh | sh
```

### Build from source

```bash
git clone https://github.com/saccofrancesco/gitsloth.git
cd gitsloth
cargo build --release
```

Binary path:

```bash
./target/release/gitsloth
```

### Setup

```bash
export OPENAI_API_KEY=your_api_key_here
```

### Usage

```bash
# Generate a single commit message (with confirmation)
./target/release/gitsloth

# Generate multiple messages and choose one
./target/release/gitsloth -g 3

# Stage everything before generating
./target/release/gitsloth -a

# Copy selected message to clipboard instead of committing
./target/release/gitsloth -c

# Skip confirmation when generating a single message
./target/release/gitsloth -y

# Print installed version
./target/release/gitsloth -v
```

---

## Flags

| Flag          | Shorthand | Description                                     |
| ------------- | --------- | ----------------------------------------------- |
| `--generate`  | `-g`      | Number of commit messages to generate           |
| `--all`       | `-a`      | Stage all changes before generating             |
| `--clipboard` | `-c`      | Copy the selected message instead of committing |
| `--yes`       | `-y`      | Skip confirmation prompt for single message     |
| `--version`   | `-v`      | Print gitsloth version                          |

---

## Examples

### Single message (default)

```bash
$ ./target/release/gitsloth
⣾ Generating commit messages...
```

```bash
Proposed message:
 feat: add clipboard support and structured git context

Confirm? (y/n):
```

### Multiple messages

```bash
$ ./target/release/gitsloth -g 3
⣾ Generating commit messages...
```

```bash
Generated messages:
1) feat: add clipboard support
2) feat: improve commit generation prompt context
3) refactor: simplify commit selection flow

Select (0 to abort):
```

---

## Development

```bash
cargo fmt
cargo clippy -- -D warnings
cargo test
```

---

## Emailware

gitsloth is [emailware](mailto:francescosacco.github@gmail.com).
If you find it useful, I would like to hear from you.

---

## License

This project is licensed under the MIT License.

---

> GitHub @saccofrancesco
