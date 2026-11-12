//! Developer tool: show the structure of an email and decode its attachments.
//! Useful for looking at calendar invitations.
//!
//! ```text
//! cargo run --example maildecoder -- -email=path/to/email.eml [options]
//!
//!   -email path        the email file (required)
//!   -out dir           output directory for -extract-all (default: attachments)
//!   -extract-all       save every part to a file
//!   -print-text=false  do not print text parts
//! ```

use std::path::{Path, PathBuf};
use std::process::ExitCode;

use mail_parser::{MessageParser, MessagePart};

const USAGE: &str = "Usage: maildecoder -email=path/to/email.eml [-out=output_dir] [-extract-all] [-print-text=false]";

struct Options {
    email: PathBuf,
    out: PathBuf,
    extract_all: bool,
    print_text: bool,
}

fn main() -> ExitCode {
    let result = parse_args(std::env::args().skip(1)).and_then(|options| run(&options));
    match result {
        Ok(()) => ExitCode::SUCCESS,
        Err(error) => {
            eprintln!("Error: {error}\n{USAGE}");
            ExitCode::FAILURE
        }
    }
}

fn parse_args(mut args: impl Iterator<Item = String>) -> Result<Options, String> {
    let mut email = None;
    let mut out = PathBuf::from("attachments");
    let mut extract_all = false;
    let mut print_text = true;

    while let Some(arg) = args.next() {
        let flag = arg.trim_start_matches('-');
        let (name, inline) = match flag.split_once('=') {
            Some((name, value)) => (name, Some(value.to_string())),
            None => (flag, None),
        };
        let mut value = || {
            inline
                .clone()
                .or_else(|| args.next())
                .ok_or(format!("flag needs an argument: -{name}"))
        };
        match name {
            "email" => email = Some(PathBuf::from(value()?)),
            "out" => out = PathBuf::from(value()?),
            "extract-all" => extract_all = inline.as_deref() != Some("false"),
            "print-text" => print_text = inline.as_deref() != Some("false"),
            _ => return Err(format!("flag provided but not defined: -{name}")),
        }
    }
    Ok(Options {
        email: email.ok_or("email path is required")?,
        out,
        extract_all,
        print_text,
    })
}

fn run(options: &Options) -> Result<(), String> {
    let raw = std::fs::read(&options.email).map_err(|e| format!("opening email file: {e}"))?;
    let message = MessageParser::default()
        .parse(&raw)
        .ok_or("parsing email: not an email")?;

    let addresses = |list: Option<mail_parser::AddressList>| {
        list.map(|list| {
            list.mailboxes()
                .filter_map(|m| m.address().map(str::to_string))
                .collect::<Vec<_>>()
                .join(", ")
        })
        .unwrap_or_default()
    };
    println!("Subject: {}", message.subject().unwrap_or_default());
    println!("From: {}", addresses(message.from()));
    println!("To: {}", addresses(message.to()));
    println!(
        "Date: {}\n",
        message.date().map(|d| d.to_rfc3339()).unwrap_or_default()
    );

    if options.extract_all {
        std::fs::create_dir_all(&options.out)
            .map_err(|e| format!("creating output directory: {e}"))?;
    }

    for (index, part) in message.parts().enumerate() {
        if part.is_multipart() {
            continue;
        }
        let media_type = media_type(&part);
        let name = file_name(&part, index + 1, &media_type);

        if options.print_text && is_text(&media_type) {
            let text = match part.text() {
                Some(text) => text.into_owned(),
                None => String::from_utf8_lossy(&part.decoded()).into_owned(),
            };
            println!("\n--- PART {}: {name} ({media_type}) ---", index + 1);
            println!("{text}");
        }
        if options.extract_all {
            let path = options.out.join(&name);
            std::fs::write(&path, part.decoded()).map_err(|e| format!("saving attachment: {e}"))?;
            println!("Saved attachment: {}", path.display());
        }
    }
    Ok(())
}

fn media_type(part: &MessagePart) -> String {
    match part.content_type() {
        Some(content_type) => format!(
            "{}/{}",
            content_type.ctype().to_ascii_lowercase(),
            content_type
                .subtype()
                .unwrap_or_default()
                .to_ascii_lowercase()
        ),
        None => "text/plain".to_string(),
    }
}

/// The attachment's own file name without any directory, or `part_N.ext`.
fn file_name(part: &MessagePart, number: usize, media_type: &str) -> String {
    part.attachment_name()
        .and_then(|name| Path::new(name).file_name())
        .map(|name| name.to_string_lossy().into_owned())
        .unwrap_or_else(|| format!("part_{number}{}", extension(media_type)))
}

fn is_text(media_type: &str) -> bool {
    media_type.starts_with("text/")
        || [
            "application/json",
            "application/xml",
            "application/javascript",
            "application/ics",
        ]
        .contains(&media_type)
}

fn extension(media_type: &str) -> &'static str {
    match media_type {
        "text/plain" => ".txt",
        "text/html" => ".html",
        "application/pdf" => ".pdf",
        "image/jpeg" => ".jpg",
        "image/png" => ".png",
        "text/calendar" | "application/ics" => ".ics",
        _ => "",
    }
}
