//! calmailproc: stores iCalendar invitations from emails on a CalDAV server.
//!
//! - [`email`] parses a mail and finds its calendar attachment.
//! - [`ical`] parses, validates, compares and writes iCalendar data.
//! - [`processor`] decides what to do with an event and talks to storage;
//!   its `stdin` and `maildir` submodules are the two input modes.
//! - [`storage`] stores events on a CalDAV server (or in memory for tests).
//! - [`cli`] and [`config`] handle flags and the config file.

pub mod cli;
pub mod config;
pub mod email;
pub mod error;
pub mod ical;
pub mod processor;
pub mod storage;
