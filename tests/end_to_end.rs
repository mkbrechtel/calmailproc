//! End-to-end tests: the real binary against a CalDAV server.
//!
//! The server here is a minimal stand-in that stores whatever is PUT and
//! serves it on GET, with basic authentication. That is all calmailproc needs
//! from CalDAV. `test.sh` runs the same scenarios against Xandikos.

use std::collections::BTreeMap;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::process::{Command, Output, Stdio};
use std::sync::{Arc, Mutex};

use calmailproc::ical;

/// "test:pass" in base64.
const AUTHORIZATION: &str = "Basic dGVzdDpwYXNz";
const CALENDAR: &str = "/user/calendars/calendar1/";

type Store = Arc<Mutex<BTreeMap<String, String>>>;

struct FakeCalDav {
    url: String,
    store: Store,
}

impl FakeCalDav {
    fn start() -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let url = format!("http://{}", listener.local_addr().unwrap());
        let store = Store::default();
        let shared = store.clone();
        std::thread::spawn(move || {
            for stream in listener.incoming().flatten() {
                let _ = handle(stream, &shared);
            }
        });
        FakeCalDav { url, store }
    }

    /// Stored calendar texts by path.
    fn events(&self) -> BTreeMap<String, String> {
        self.store.lock().unwrap().clone()
    }

    fn args(&self) -> Vec<String> {
        [
            "-process-replies",
            "-url",
            &self.url,
            "-user",
            "test",
            "-pass",
            "pass",
            "-calendar",
            CALENDAR,
        ]
        .map(String::from)
        .to_vec()
    }
}

fn handle(stream: TcpStream, store: &Store) -> std::io::Result<()> {
    let mut reader = BufReader::new(stream.try_clone()?);
    let mut request_line = String::new();
    reader.read_line(&mut request_line)?;
    let mut parts = request_line.split_whitespace();
    let (method, path) = (parts.next().unwrap_or(""), parts.next().unwrap_or(""));

    let mut content_length = 0;
    let mut authorized = false;
    loop {
        let mut line = String::new();
        reader.read_line(&mut line)?;
        let line = line.trim_end();
        if line.is_empty() {
            break;
        }
        if let Some((name, value)) = line.split_once(':') {
            match name.to_ascii_lowercase().as_str() {
                "content-length" => content_length = value.trim().parse().unwrap_or(0),
                "authorization" => authorized = value.trim() == AUTHORIZATION,
                _ => {}
            }
        }
    }
    let mut body = vec![0; content_length];
    reader.read_exact(&mut body)?;

    let (status, response_body) = if !authorized {
        ("401 Unauthorized", String::new())
    } else {
        let mut store = store.lock().unwrap();
        match method {
            "PUT" => {
                store.insert(
                    path.to_string(),
                    String::from_utf8_lossy(&body).into_owned(),
                );
                ("201 Created", String::new())
            }
            "GET" => match store.get(path) {
                Some(ics) => ("200 OK", ics.clone()),
                None => ("404 Not Found", String::new()),
            },
            _ => ("405 Method Not Allowed", String::new()),
        }
    };
    let mut stream = stream;
    write!(
        stream,
        "HTTP/1.1 {status}\r\nContent-Type: text/calendar\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response_body}",
        response_body.len()
    )
}

/// A fresh, empty directory to use as XDG_CONFIG_HOME, so the tests never
/// read the config file of whoever runs them.
fn config_home(name: &str) -> PathBuf {
    let dir = std::env::temp_dir().join(format!("calmailproc-e2e-{}-{name}", std::process::id()));
    let _ = std::fs::remove_dir_all(&dir);
    std::fs::create_dir_all(dir.join("calmailproc")).unwrap();
    dir
}

fn run(args: &[String], stdin: Option<&[u8]>, config_home: &PathBuf) -> Output {
    let mut child = Command::new(env!("CARGO_BIN_EXE_calmailproc"))
        .args(args)
        .env("XDG_CONFIG_HOME", config_home)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .unwrap();
    // A closed pipe is fine: the binary may exit without reading stdin.
    let _ = child
        .stdin
        .take()
        .unwrap()
        .write_all(stdin.unwrap_or_default());
    child.wait_with_output().unwrap()
}

fn text(bytes: &[u8]) -> String {
    String::from_utf8_lossy(bytes).into_owned()
}

fn test_mails() -> Vec<PathBuf> {
    let mut mails: Vec<PathBuf> = std::fs::read_dir("test/maildir/cur")
        .unwrap()
        .map(|entry| entry.unwrap().path())
        .filter(|path| path.extension().is_some_and(|ext| ext == "eml"))
        .collect();
    mails.sort();
    mails
}

/// Feeds every test mail to the binary on stdin, like a mail filter would.
fn run_stdin_mode(server: &FakeCalDav, config_home: &PathBuf) {
    for mail in test_mails() {
        let name = mail.file_name().unwrap().to_string_lossy().into_owned();
        let output = run(
            &server.args(),
            Some(&std::fs::read(&mail).unwrap()),
            config_home,
        );
        // Tests 12-15 contain invalid calendar data and must be rejected.
        let expected = if ["test-12.eml", "test-13.eml", "test-14.eml", "test-15.eml"]
            .contains(&name.as_str())
        {
            1
        } else {
            0
        };
        assert_eq!(
            output.status.code(),
            Some(expected),
            "{name}: stdout: {} stderr: {}",
            text(&output.stdout),
            text(&output.stderr)
        );
        if expected == 1 {
            assert!(text(&output.stderr).starts_with("Error: error processing stdin: "));
            assert!(output.stdout.is_empty());
        } else {
            assert_eq!(text(&output.stdout).lines().count(), 1, "one status line");
        }
    }
}

#[test]
fn stdin_maildir_and_config_file_modes_store_the_same_events() {
    let empty_config = config_home("empty");

    // Mode 1: one mail at a time on stdin.
    let stdin_server = FakeCalDav::start();
    run_stdin_mode(&stdin_server, &empty_config);
    let stdin_events = stdin_server.events();
    assert_eq!(stdin_events.len(), 14);

    // Mode 2: the whole maildir in one run.
    let maildir_server = FakeCalDav::start();
    let mut args = maildir_server.args();
    args.extend(["-maildir", "test/maildir", "-verbose"].map(String::from));
    let output = run(&args, None, &empty_config);
    assert_eq!(output.status.code(), Some(0), "{}", text(&output.stderr));
    let stderr = text(&output.stderr);
    assert_eq!(
        stderr.matches("Warning: failed to process").count(),
        4,
        "{stderr}"
    );
    assert_eq!(maildir_server.events(), stdin_events);

    // Mode 3: all settings from the config file.
    let config_server = FakeCalDav::start();
    let with_config = config_home("config");
    std::fs::write(
        with_config.join("calmailproc").join("config.yaml"),
        format!(
            "webdav:\n  url: {}\n  user: test\n  pass: pass\n  calendar: {CALENDAR}\n\nprocessor:\n  process_replies: true\n\nmaildir:\n  path: test/maildir\n  verbose: false\n",
            config_server.url
        ),
    )
    .unwrap();
    let output = run(&[], None, &with_config);
    assert_eq!(output.status.code(), Some(0), "{}", text(&output.stderr));
    assert_eq!(config_server.events(), stdin_events);

    // What ended up on the server.
    for (path, ics) in &stdin_events {
        assert!(
            path.starts_with(CALENDAR) && path.ends_with(".ics"),
            "{path}"
        );
        let calendar = ical::decode(ics).unwrap();
        ical::validate(&calendar).unwrap();
        assert!(
            !calendar.has_prop("METHOD"),
            "{path}: METHOD must not be stored"
        );
        // Otherwise a server with scheduling would mail the participants again.
        for participant in calendar
            .events()
            .flat_map(|vevent| vevent.properties.iter())
            .filter(|p| p.name == "ORGANIZER" || p.name == "ATTENDEE")
        {
            assert_eq!(
                participant.param("SCHEDULE-AGENT"),
                Some("CLIENT"),
                "{path}: {} {}",
                participant.name,
                participant.value
            );
        }
    }
    let sequence_of = |uid_prefix: &str| {
        let (_, ics) = stdin_events
            .iter()
            .find(|(path, _)| path.contains(uid_prefix))
            .unwrap();
        ical::parse_ical_data(ics).unwrap().sequence
    };
    // Test 07: updates arrive as sequence 0, 3, 4, 2, 1. The server must keep 4,
    // which only works if the stored sequence is read back correctly.
    assert_eq!(sequence_of("FD5688352A1F9347AAD14BB6A7C46727"), 4);
    // Tests 05 and 06: replies update the invitation instead of replacing it.
    let (_, test_5) = stdin_events
        .iter()
        .find(|(path, _)| path.contains("0e4a0959"))
        .unwrap();
    assert!(test_5.contains("SUMMARY:Test 5"));
    assert!(test_5.contains("PARTSTAT=ACCEPTED"));

    let _ = std::fs::remove_dir_all(empty_config);
    let _ = std::fs::remove_dir_all(with_config);
}

#[test]
fn wrong_password_is_an_error() {
    let server = FakeCalDav::start();
    let config = config_home("auth");
    let mut args = server.args();
    let pass = args.iter().position(|arg| arg == "pass").unwrap();
    args[pass] = "wrong".to_string();
    let mail = std::fs::read("test/maildir/cur/test-01-1.eml").unwrap();

    let output = run(&args, Some(&mail), &config);
    assert_eq!(output.status.code(), Some(1));
    let stderr = text(&output.stderr);
    assert!(stderr.contains("looking up existing event"), "{stderr}");
    assert!(stderr.contains("401"), "{stderr}");
    assert!(server.events().is_empty());
    let _ = std::fs::remove_dir_all(config);
}

#[test]
fn command_line_errors_and_help() {
    let config = config_home("cli");
    let args = |args: &[&str]| args.iter().map(|s| s.to_string()).collect::<Vec<_>>();

    let output = run(&args(&["-help"]), None, &config);
    assert_eq!(output.status.code(), Some(0));
    assert!(text(&output.stdout).starts_with("Usage of calmailproc:"));

    let output = run(&args(&["-version"]), None, &config);
    assert_eq!(
        text(&output.stdout).trim(),
        format!("calmailproc {}", env!("CARGO_PKG_VERSION"))
    );

    let output = run(&args(&["-caldav", "http://example.com"]), None, &config);
    assert_eq!(output.status.code(), Some(2));
    assert!(text(&output.stderr).starts_with("flag provided but not defined: -caldav"));

    let output = run(&[], Some(b"Subject: nothing\r\n\r\nbody\r\n"), &config);
    assert_eq!(output.status.code(), Some(1));
    assert_eq!(
        text(&output.stderr).trim(),
        "Error: all CalDAV flags are required: -url, -user, -pass, -calendar"
    );

    // A broken config file is a warning; the flags are still required then.
    std::fs::write(
        config.join("calmailproc").join("config.yaml"),
        "webdav: [unclosed",
    )
    .unwrap();
    let output = run(&[], None, &config);
    assert_eq!(output.status.code(), Some(1));
    assert!(text(&output.stderr).starts_with("Warning: failed to load config file: "));
    let _ = std::fs::remove_dir_all(config);
}
