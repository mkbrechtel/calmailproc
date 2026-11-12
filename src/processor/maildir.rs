//! Maildir mode: process every email in a maildir and its subfolders.
//!
//! A mail that cannot be processed is reported as a warning and does not stop
//! the run.

use std::fs;
use std::path::Path;

use serde::Deserialize;

use super::{NO_CALENDAR_EVENT, Processor};
use crate::error::{Context, Error, Result};
use crate::storage::Storage;

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct MaildirConfig {
    pub path: String,
    pub verbose: bool,
}

/// Files that live in maildir folders but are not emails.
const NOT_EMAILS: [&str; 3] = [".DS_Store", "maildirfolder", ".uidvalidity"];

pub fn process_with_config<S: Storage>(
    config: &MaildirConfig,
    processor: &mut Processor<S>,
) -> Result<()> {
    process(Path::new(&config.path), processor, config.verbose)
}

pub fn process<S: Storage>(
    maildir: &Path,
    processor: &mut Processor<S>,
    verbose: bool,
) -> Result<()> {
    if verbose {
        eprintln!("Starting to process maildir: {}", maildir.display());
    }
    if !maildir.exists() {
        return Err(Error::new(format!(
            "maildir path does not exist: {}",
            maildir.display()
        )));
    }
    if verbose {
        let entries = sorted_entries(maildir).context("reading maildir")?;
        eprintln!("Found {} entries in maildir root", entries.len());
        for (name, is_dir) in &entries {
            eprintln!("  - {name} (dir: {is_dir})");
        }
    }
    process_directory(maildir, processor, verbose)
        .context(&format!("processing maildir {}", maildir.display()))
}

/// Processes the `new` and `cur` folders of a directory, then its subfolders.
fn process_directory<S: Storage>(
    dir: &Path,
    processor: &mut Processor<S>,
    verbose: bool,
) -> Result<()> {
    for folder in ["new", "cur"] {
        let path = dir.join(folder);
        if path.is_dir() {
            if verbose {
                eprintln!("Processing '{folder}' directory: {}", path.display());
            }
            process_emails(&path, processor, verbose)
                .context(&format!("processing '{folder}' directory"))?;
        } else if verbose {
            eprintln!("'{folder}' directory does not exist: {}", path.display());
        }
    }
    process_subdirectories(dir, processor, verbose)
}

fn process_emails<S: Storage>(
    dir: &Path,
    processor: &mut Processor<S>,
    verbose: bool,
) -> Result<()> {
    let entries = sorted_entries(dir)?;
    if verbose {
        eprintln!(
            "Found {} files in directory: {}",
            entries.len(),
            dir.display()
        );
    }

    let mut processed = 0;
    for (name, is_dir) in &entries {
        if *is_dir || NOT_EMAILS.contains(&name.as_str()) {
            continue;
        }
        match process_email_file(&dir.join(name), processor, verbose) {
            Ok(()) => processed += 1,
            Err(error) => eprintln!("Warning: {error}"),
        }
    }

    if verbose {
        eprintln!(
            "Processed {processed}/{} files in: {}",
            entries.len(),
            dir.display()
        );
    }
    Ok(())
}

fn process_email_file<S: Storage>(
    path: &Path,
    processor: &mut Processor<S>,
    verbose: bool,
) -> Result<()> {
    let raw = fs::read(path).context(&format!("failed to open {}", path.display()))?;
    let outcome = processor.process_email(&raw);
    let status = match &outcome {
        Ok(status) => status,
        Err(failure) => &failure.status,
    };
    if verbose || status != NO_CALENDAR_EVENT {
        println!("{} > {status}", path.display());
    }
    outcome
        .map(|_| ())
        .map_err(|failure| failure.error)
        .context(&format!("failed to process {}", path.display()))
}

fn process_subdirectories<S: Storage>(
    parent: &Path,
    processor: &mut Processor<S>,
    verbose: bool,
) -> Result<()> {
    if verbose {
        eprintln!("Looking for subfolders in: {}", parent.display());
    }
    for (name, is_dir) in sorted_entries(parent)? {
        if !is_dir || ["new", "cur", "tmp"].contains(&name.as_str()) {
            continue;
        }
        let path = parent.join(&name);
        if verbose {
            let kind = if path.join("cur").is_dir() || path.join("new").is_dir() {
                "maildir"
            } else {
                "non-maildir"
            };
            eprintln!("Processing {kind} subfolder: {}", path.display());
        }
        // A broken subfolder must not stop the others.
        if let Err(error) = process_directory(&path, processor, verbose)
            && verbose
        {
            eprintln!(
                "Warning: Error processing directory {}: {error}",
                path.display()
            );
        }
    }
    Ok(())
}

/// Directory entries as (name, is directory), sorted by name so that mails are
/// processed in a stable order.
fn sorted_entries(dir: &Path) -> Result<Vec<(String, bool)>> {
    let context = format!("reading directory {}", dir.display());
    let mut entries = Vec::new();
    for entry in fs::read_dir(dir).context(&context)? {
        let entry = entry.context(&context)?;
        entries.push((
            entry.file_name().to_string_lossy().into_owned(),
            entry.path().is_dir(),
        ));
    }
    entries.sort();
    Ok(entries)
}
