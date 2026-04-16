<div align="center">
  <h1>gitsloth: AI-Powered Git Commit Messages</h1>
</div>

<p align="center">
  <a href="https://buymeacoffee.com/saccofrancesco">
    <img src="https://img.shields.io/badge/Buy%20Me%20a%20Coffee-ffdd00?style=for-the-badge&logo=buy-me-a-coffee&logoColor=black" />
  </a>
</p>

<h4 align="center">
A lightweight CLI tool that generates clean, Conventional Commit messages from your staged Git changes using OpenAI — fast, minimal, and dependency-free.
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

## 📌 TL;DR

gitsloth is a minimal Go CLI that reads your staged Git diff (along with your branch name and status), generates one or more Conventional Commit messages using OpenAI, lets you pick the best one, and then commits it — or copies it to your clipboard.

---

## 💡 Why gitsloth

- Writing good commit messages is repetitive  
- Conventional Commits are useful but tedious  
- This tool automates it **without adding complexity**

It focuses on:
- **zero unnecessary dependencies**
- **simple UX**
- **predictable output**

---

## 🔑 Key Features

* **AI-Generated Commit Messages** – Uses OpenAI to turn diffs into clean Conventional Commits  
* **Multiple Suggestions** – Generate several commit messages and pick the best one  
* **Rich Git Context** – Sends branch name, status, and diff together for better accuracy  
* **Conventional Commits Ready** – Enforces proper format and style automatically  
* **Interactive Selection** – Choose your preferred message when generating multiple options  
* **Confirmation Flow** – Confirm before committing when generating a single message  
* **Clipboard Support** – Copy the selected message instead of committing  
* **Zero Dependencies** – Uses only the Go standard library  
* **Fast CLI Workflow** – Designed to fit seamlessly into your Git routine  

---

## ⚡ Quickstart

```bash
git clone https://github.com/saccofrancesco/gitsloth.git
cd gitsloth
go build -o gitsloth
````

### Setup

```bash
export OPENAI_API_KEY=your_api_key_here
```

### Usage

```bash
# Generate a single commit message (with confirmation)
./gitsloth

# Generate multiple messages and choose one
./gitsloth -g 3

# Stage everything before generating
./gitsloth -a

# Copy selected message to clipboard instead of committing
./gitsloth -c
```

---

## 🧾 Flags

| Flag          | Shorthand | Description                                     |
| ------------- | --------- | ----------------------------------------------- |
| `--generate`  | `-g`      | Number of commit messages to generate           |
| `--all`       | `-a`      | Stage all changes before generating             |
| `--clipboard` | `-c`      | Copy the selected message instead of committing |

---

## 🧪 Examples

### Single message (default)

```bash
$ ./gitsloth
⣾ Generating commit message...
```

```bash
Proposed commit message:
feat: add clipboard support and structured git context

Accept and commit? (y/n):
```

---

### Multiple messages

```bash
$ ./gitsloth -g 3
⣾ Generating commit messages...
```

```bash
Generated commit messages:
1) feat: add clipboard support
2) feat: implement clipboard integration for commits
3) refactor: improve commit message generation flow

Select a message (number) or 0 to abort:
```

---

## 📬 Emailware: Share Your Thoughts

gitsloth is [emailware](mailto:francescosacco.github@gmail.com).
If you find it useful or interesting, I'd like to hear from you.

📩 [francescosacco.github@gmail.com](mailto:francescosacco.github@gmail.com)

---

## 🙏 Support

If you like this project:

* ⭐️ Star the repo
* ☕️ Buy me a coffee
* 💌 Send feedback or ideas

---

📎 You Might Also Like…

* Deepshot: Predict NBA games using machine learning and advanced stats
* Supremebot: A NiceGUI-powered bot for Supreme drops

---

## 📜 License

This project is licensed under the MIT License — feel free to use it in your own projects!

---

> GitHub @saccofrancesco