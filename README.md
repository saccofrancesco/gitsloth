<div align="center">
  <h1>gitsloth: AI-Powered Git Commit Messages</h1>
</div>

<p align="center">
  <a href="https://www.buymeacoffee.com/saccofrancesco">
    <img src="https://img.buymeacoffee.com/button-api/?text=Buy me a coffee&emoji=☕&slug=saccofrancesco&button_colour=FFDD00&font_colour=000000&font_family=Bree&outline_colour=000000&coffee_colour=ffffff" />
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

gitsloth is a minimal Go CLI that reads your staged Git diff (along with your branch name and status), generates a Conventional Commit message using OpenAI, and asks for confirmation before committing — or copies the message to your clipboard.

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
* **Rich Git Context** – Sends branch name, status, and diff together for more accurate messages
* **Conventional Commits Ready** – Enforces proper format and style automatically
* **Interactive Confirmation** – You always approve before committing
* **Clipboard Support** – Copy the generated message instead of committing directly
* **Zero Dependencies** – Uses only the Go standard library
* **Fast CLI Workflow** – Designed to fit seamlessly into your Git routine

---

## ⚡ Quickstart

```bash
git clone https://github.com/saccofrancesco/gitsloth.git
cd gitsloth
go build -o gitsloth
```

### Setup

```bash
export OPENAI_API_KEY=your_api_key_here
```

### Usage

```bash
# Stage your changes, then generate a commit message
git add .
./gitsloth

# Let the tool stage everything for you
./gitsloth -a

# Copy the generated message to clipboard instead of committing
./gitsloth -c
```

| Flag | Shorthand | Description |
|------|-----------|-------------|
| `--all` | `-a` | Stage all changes before generating the commit |
| `--clipboard` | `-c` | Copy the generated message to clipboard instead of committing |

---

## 🧪 Example

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

## 📬 Emailware: Share Your Thoughts

gitsloth is [emailware](mailto:francescosacco.github@gmail.com).  
If you find it useful or interesting, I'd like to hear from you.

📩 francescosacco.github@gmail.com

---

## 🙏 Support

If you like this project:

* ⭐️ Star the repo
* ☕️ [Buy me a coffee](https://www.buymeacoffee.com/saccofrancesco)
* 💌 Send feedback or ideas

---

📎 You Might Also Like…

* [Deepshot](https://github.com/saccofrancesco/deepshot): Predict NBA games using machine learning and advanced stats.
* [Supremebot](https://github.com/saccofrancesco/supremebot): A NiceGUI-powered bot for Supreme drops.

---

## 📜 License

This project is licensed under the [MIT License](https://opensource.org/licenses/MIT) — feel free to use it in your own projects!

---

> GitHub [@saccofrancesco](https://github.com/saccofrancesco)