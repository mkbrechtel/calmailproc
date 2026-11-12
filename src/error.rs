//! One error type for the whole application.
//!
//! Errors are only ever reported as text, so this is a message with the same
//! "context: cause" chaining the Go version got from `fmt.Errorf("...: %w")`.

use std::fmt;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Error(String);

pub type Result<T> = std::result::Result<T, Error>;

impl Error {
    pub fn new(message: impl Into<String>) -> Self {
        Error(message.into())
    }
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

impl std::error::Error for Error {}

impl From<String> for Error {
    fn from(message: String) -> Self {
        Error(message)
    }
}

impl From<&str> for Error {
    fn from(message: &str) -> Self {
        Error(message.to_string())
    }
}

/// Adds context in front of an error: `result.context("parsing email")?`
/// turns `cause` into `parsing email: cause`.
pub trait Context<T> {
    fn context(self, context: &str) -> Result<T>;
}

impl<T, E: fmt::Display> Context<T> for std::result::Result<T, E> {
    fn context(self, context: &str) -> Result<T> {
        self.map_err(|cause| Error(format!("{context}: {cause}")))
    }
}
