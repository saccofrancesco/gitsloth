use serde_json::{json, Value};
use std::env;
use std::fs;
use std::io::{self, Write};
use std::process::{Command, Stdio};
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};
use std::thread;
use std::time::Duration;

#[derive(Debug, Clone, Copy)]
struct CliOptions {
    all: bool,
    clipboard: bool,
    generate: i32,
    yes: bool,
}

#[derive(Debug)]
enum ParseOutcome {
    Continue(CliOptions),
    Exit(i32),
}

#[derive(Debug)]
struct GitContext {
    branch: String,
    status: String,
    diff: String,
}

#[derive(Debug)]
struct Spinner {
    running: Arc<AtomicBool>,
    handle: Option<thread::JoinHandle<()>>,
}

impl Spinner {
    fn start(message: &str) -> Self {
        let running = Arc::new(AtomicBool::new(true));
        let worker_running = Arc::clone(&running);
        let msg = message.to_string();

        let handle = thread::spawn(move || {
            let chars = ['⣷', '⣯', '⣟', '⡿', '⢿', '⣻', '⣽', '⣾'];
            let mut idx = 0usize;

            while worker_running.load(Ordering::Relaxed) {
                print!("\r{} {}", chars[idx % chars.len()], msg);
                let _ = io::stdout().flush();
                thread::sleep(Duration::from_millis(100));
                idx = idx.wrapping_add(1);
            }

            print!("\r\x1B[K");
            let _ = io::stdout().flush();
        });

        Self {
            running,
            handle: Some(handle),
        }
    }

    fn stop(mut self) {
        self.running.store(false, Ordering::Relaxed);
        if let Some(handle) = self.handle.take() {
            let _ = handle.join();
        }
    }
}

const CONVENTIONAL_COMMIT_RULES: &str = r#"
Use Conventional Commits:

<type>(optional scope): <summary>

Types:
feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert

Summary rules:
- lowercase
- no trailing period
- max 72 chars
- imperative mood
"#;

fn main() {
    std::process::exit(run());
}

fn run() -> i32 {
    let args: Vec<String> = env::args().collect();
    let program = args.first().map_or("gitsloth", String::as_str);

    let options = match parse_flags(&args[1..], program) {
        ParseOutcome::Continue(options) => options,
        ParseOutcome::Exit(code) => return code,
    };

    if options.generate < 1 {
        println!("generate must be >= 1");
        return 1;
    }

    if !is_git_repo_here() {
        println!("Not inside a Git repository (.git not found here)");
        return 1;
    }

    if options.all {
        if let Err(err) = stage_all_changes() {
            println!("Failed to stage changes: {err}");
            return 1;
        }
    }

    let ctx = match build_git_context() {
        Ok(ctx) => ctx,
        Err(err) => {
            println!("Failed to build git context: {err}");
            return 1;
        }
    };

    if ctx.diff.trim().is_empty() {
        println!("No changes to commit");
        return 0;
    }

    let messages = match generate_commit_messages(&ctx, options.generate as usize) {
        Ok(messages) => messages,
        Err(err) => {
            println!("Failed to generate commit messages: {err}");
            return 1;
        }
    };

    let Some(message) = select_message(&messages) else {
        println!("Operation aborted");
        return 0;
    };

    if options.generate == 1 && !options.yes && !ask_for_confirmation(&message) {
        println!("Operation aborted");
        return 0;
    }

    if options.clipboard {
        if let Err(err) = copy_to_clipboard(&message) {
            println!("Failed to copy to clipboard: {err}");
            return 1;
        }
        println!("Message copied to clipboard");
        return 0;
    }

    if let Err(err) = create_commit(&message) {
        println!("{err}");
        return 1;
    }

    0
}

fn parse_flags(args: &[String], program: &str) -> ParseOutcome {
    let mut options = CliOptions {
        all: false,
        clipboard: false,
        generate: 1,
        yes: false,
    };

    let mut i = 0usize;
    while i < args.len() {
        let arg = &args[i];

        if arg == "--" {
            break;
        }

        if arg == "-h" || arg == "--h" || arg == "-help" || arg == "--help" {
            let mut out = io::stdout();
            print_usage(program, &mut out);
            return ParseOutcome::Exit(0);
        }

        if !arg.starts_with('-') || arg == "-" {
            break;
        }

        let trimmed = if let Some(rest) = arg.strip_prefix("--") {
            rest
        } else {
            &arg[1..]
        };

        let (name, inline_value) = match trimmed.split_once('=') {
            Some((name, value)) => (name, Some(value.to_string())),
            None => (trimmed, None),
        };

        match name {
            "a" | "all" => {
                let value = inline_value.as_deref().unwrap_or("true");
                match parse_go_bool(value) {
                    Ok(parsed) => options.all = parsed,
                    Err(err) => {
                        eprintln!("invalid boolean value {value:?} for -{name}: {err}");
                        let mut out = io::stderr();
                        print_usage(program, &mut out);
                        return ParseOutcome::Exit(2);
                    }
                }
            }
            "c" | "clipboard" => {
                let value = inline_value.as_deref().unwrap_or("true");
                match parse_go_bool(value) {
                    Ok(parsed) => options.clipboard = parsed,
                    Err(err) => {
                        eprintln!("invalid boolean value {value:?} for -{name}: {err}");
                        let mut out = io::stderr();
                        print_usage(program, &mut out);
                        return ParseOutcome::Exit(2);
                    }
                }
            }
            "y" | "yes" => {
                let value = inline_value.as_deref().unwrap_or("true");
                match parse_go_bool(value) {
                    Ok(parsed) => options.yes = parsed,
                    Err(err) => {
                        eprintln!("invalid boolean value {value:?} for -{name}: {err}");
                        let mut out = io::stderr();
                        print_usage(program, &mut out);
                        return ParseOutcome::Exit(2);
                    }
                }
            }
            "g" | "generate" => {
                let raw_value = if let Some(value) = inline_value {
                    value
                } else {
                    i += 1;
                    if i >= args.len() {
                        eprintln!("flag needs an argument: -{name}");
                        let mut out = io::stderr();
                        print_usage(program, &mut out);
                        return ParseOutcome::Exit(2);
                    }
                    args[i].clone()
                };

                match raw_value.parse::<i32>() {
                    Ok(parsed) => options.generate = parsed,
                    Err(_) => {
                        eprintln!("invalid value {raw_value:?} for flag -{name}: parse error");
                        let mut out = io::stderr();
                        print_usage(program, &mut out);
                        return ParseOutcome::Exit(2);
                    }
                }
            }
            _ => {
                eprintln!("flag provided but not defined: -{name}");
                let mut out = io::stderr();
                print_usage(program, &mut out);
                return ParseOutcome::Exit(2);
            }
        }

        i += 1;
    }

    ParseOutcome::Continue(options)
}

fn parse_go_bool(input: &str) -> Result<bool, &'static str> {
    match input {
        "1" | "t" | "T" | "true" | "TRUE" | "True" => Ok(true),
        "0" | "f" | "F" | "false" | "FALSE" | "False" => Ok(false),
        _ => Err("parse error"),
    }
}

fn print_usage(program: &str, out: &mut dyn Write) {
    let _ = writeln!(out, "Usage of {program}:");
    let _ = writeln!(out, "  -a\tshorthand for --all");
    let _ = writeln!(out, "  -all");
    let _ = writeln!(out, "    \tstage all changes before committing");
    let _ = writeln!(out, "  -c\tshorthand for --clipboard");
    let _ = writeln!(out, "  -clipboard");
    let _ = writeln!(out, "    \tcopy selected message to clipboard");
    let _ = writeln!(out, "  -g int");
    let _ = writeln!(out, "    \tshorthand for --generate (default 1)");
    let _ = writeln!(out, "  -generate int");
    let _ = writeln!(
        out,
        "    \tnumber of commit messages to generate (default 1)"
    );
    let _ = writeln!(out, "  -y\tshorthand for --yes");
    let _ = writeln!(out, "  -yes");
    let _ = writeln!(out, "    \tskip confirmation prompt");
}

fn is_git_repo_here() -> bool {
    let cwd = match env::current_dir() {
        Ok(path) => path,
        Err(_) => return false,
    };

    fs::metadata(cwd.join(".git")).is_ok()
}

fn stage_all_changes() -> Result<(), String> {
    let out = Command::new("git")
        .args(["add", "-A"])
        .output()
        .map_err(|err| err.to_string())?;

    if out.status.success() {
        Ok(())
    } else {
        let mut combined = String::new();
        combined.push_str(&String::from_utf8_lossy(&out.stdout));
        combined.push_str(&String::from_utf8_lossy(&out.stderr));
        Err(format!("git add failed: {combined}"))
    }
}

fn is_command_available(name: &str) -> bool {
    let path_var = match env::var_os("PATH") {
        Some(path_var) => path_var,
        None => return false,
    };

    let paths = env::split_paths(&path_var);
    #[cfg(windows)]
    let exts = env::var_os("PATHEXT")
        .map(|v| {
            v.to_string_lossy()
                .split(';')
                .map(|s| s.trim_matches('.').to_string())
                .collect::<Vec<_>>()
        })
        .unwrap_or_else(|| vec!["EXE".to_string(), "BAT".to_string(), "CMD".to_string()]);

    for dir in paths {
        let candidate = dir.join(name);
        if candidate.is_file() {
            return true;
        }

        #[cfg(windows)]
        {
            for ext in &exts {
                let with_ext = dir.join(format!("{name}.{}", ext.to_ascii_lowercase()));
                let with_upper_ext = dir.join(format!("{name}.{}", ext.to_ascii_uppercase()));
                if with_ext.is_file() || with_upper_ext.is_file() {
                    return true;
                }
            }
        }
    }

    false
}

fn copy_to_clipboard(text: &str) -> Result<(), String> {
    let mut cmd = if is_command_available("pbcopy") {
        Command::new("pbcopy")
    } else if is_command_available("xclip") {
        let mut c = Command::new("xclip");
        c.args(["-selection", "clipboard"]);
        c
    } else if is_command_available("wl-copy") {
        Command::new("wl-copy")
    } else if is_command_available("clip") {
        let mut c = Command::new("cmd");
        c.args(["/c", "clip"]);
        c
    } else {
        return Err("no clipboard utility found".to_string());
    };

    let mut child = cmd
        .stdin(Stdio::piped())
        .spawn()
        .map_err(|err| err.to_string())?;

    if let Some(stdin) = child.stdin.as_mut() {
        stdin
            .write_all(text.as_bytes())
            .map_err(|err| err.to_string())?;
    }

    let status = child.wait().map_err(|err| err.to_string())?;
    if status.success() {
        Ok(())
    } else {
        Err(format!("clipboard command exited with status {status}"))
    }
}

fn run_git_output(args: &[&str]) -> Result<String, String> {
    let out = Command::new("git")
        .args(args)
        .output()
        .map_err(|err| err.to_string())?;

    if !out.status.success() {
        return Err(format!("git command failed with status {}", out.status));
    }

    Ok(String::from_utf8_lossy(&out.stdout).trim().to_string())
}

fn get_branch_name() -> Result<String, String> {
    run_git_output(&["rev-parse", "--abbrev-ref", "HEAD"])
}

fn get_short_git_status() -> Result<String, String> {
    run_git_output(&["status", "--short"])
}

fn get_truncated_diff(max: usize) -> Result<String, String> {
    let out = Command::new("git")
        .args(["diff", "--cached"])
        .output()
        .map_err(|err| err.to_string())?;

    if !out.status.success() {
        return Err(format!("git command failed with status {}", out.status));
    }

    let mut diff = String::from_utf8_lossy(&out.stdout).to_string();
    if diff.len() > max {
        diff.truncate(max);
        diff.push_str("\n... (truncated)");
    }
    Ok(diff)
}

fn build_git_context() -> Result<GitContext, String> {
    let branch = get_branch_name()?;
    let status = get_short_git_status()?;
    let diff = get_truncated_diff(8000)?;

    Ok(GitContext {
        branch,
        status,
        diff,
    })
}

fn generate_commit_messages(ctx: &GitContext, count: usize) -> Result<Vec<String>, String> {
    let key = env::var("OPENAI_API_KEY").map_err(|_| "OPENAI_API_KEY not set".to_string())?;

    let spinner = Spinner::start("Generating commit messages...");

    let prompt = format!(
        "\n{}\n\nBranch:\n{}\n\nStatus:\n{}\n\nDiff:\n{}\n\nGenerate {} commit messages as JSON array of strings.",
        CONVENTIONAL_COMMIT_RULES, ctx.branch, ctx.status, ctx.diff, count
    );

    let request_body = json!({
        "model": "gpt-4o-mini",
        "messages": [
            {
                "role": "user",
                "content": prompt,
            }
        ],
    });

    let response = ureq::post("https://api.openai.com/v1/chat/completions")
        .set("Authorization", &format!("Bearer {key}"))
        .set("Content-Type", "application/json")
        .send_string(&request_body.to_string());

    spinner.stop();

    let response = response.map_err(|err| err.to_string())?;
    let content = response.into_string().map_err(|err| err.to_string())?;
    let parsed: Value = serde_json::from_str(&content).map_err(|err| err.to_string())?;

    let content = parsed
        .get("choices")
        .and_then(Value::as_array)
        .and_then(|choices| choices.first())
        .and_then(|first| first.get("message"))
        .and_then(|message| message.get("content"))
        .and_then(Value::as_str)
        .ok_or_else(|| "missing choices[0].message.content in OpenAI response".to_string())?;

    let cleaned = content.trim().replace("```json", "").replace("```", "");

    let messages: Vec<String> = serde_json::from_str(&cleaned).map_err(|err| err.to_string())?;

    Ok(messages)
}

fn select_message(messages: &[String]) -> Option<String> {
    if messages.len() == 1 {
        return messages.first().cloned();
    }

    println!("Generated messages:");
    for (idx, message) in messages.iter().enumerate() {
        println!("{}) {}", idx + 1, message);
    }
    print!("Select (0 to abort): ");
    let _ = io::stdout().flush();

    let mut input = String::new();
    if io::stdin().read_line(&mut input).is_err() {
        return None;
    }

    let choice = input.trim().parse::<usize>().unwrap_or(0);
    if choice == 0 || choice > messages.len() {
        None
    } else {
        Some(messages[choice - 1].clone())
    }
}

fn ask_for_confirmation(message: &str) -> bool {
    println!("Proposed message:\n {message}");
    print!("Confirm? (y/n): ");
    let _ = io::stdout().flush();

    let mut input = String::new();
    if io::stdin().read_line(&mut input).is_err() {
        return false;
    }

    matches!(input.trim().to_ascii_lowercase().as_str(), "y" | "yes")
}

fn create_commit(message: &str) -> Result<(), String> {
    let out = Command::new("git")
        .args(["commit", "-m", message])
        .output()
        .map_err(|err| err.to_string())?;

    if out.status.success() {
        print!("{}", String::from_utf8_lossy(&out.stdout));
        Ok(())
    } else {
        let mut combined = String::new();
        combined.push_str(&String::from_utf8_lossy(&out.stdout));
        combined.push_str(&String::from_utf8_lossy(&out.stderr));
        Err(format!("commit failed: {combined}"))
    }
}
