//! Stdin mode: process a single email and print what happened.

use std::io::Read;

use super::Processor;
use crate::error::{Context, Error, Result};
use crate::storage::Storage;

/// Processes one email from standard input.
pub fn process<S: Storage>(processor: &mut Processor<S>) -> Result<()> {
    process_reader(std::io::stdin().lock(), processor).context("processing stdin")
}

/// Processes one email from any reader and prints the status message.
pub fn process_reader<S: Storage>(
    mut reader: impl Read,
    processor: &mut Processor<S>,
) -> Result<()> {
    let mut raw = Vec::new();
    reader.read_to_end(&mut raw).context("reading email")?;
    let status = processor
        .process_email(&raw)
        .map_err(|failure: super::Failure| -> Error { failure.error })?;
    println!("{status}");
    Ok(())
}
