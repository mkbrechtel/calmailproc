# Issues

This directory contains issue reports and feature requests for calmailproc. Issues are tracked as markdown files to provide better version control, searchability, and documentation.

## How to Report an Issue

1. **Create a new markdown file** in this directory with a descriptive name:
   - For bugs: `bug-description.md` (e.g., `bug-caldav-connection-timeout.md`)
   - For features: `feature-description.md` (e.g., `feature-imap-support.md`)
   - For improvements: `improvement-description.md`

2. **Document the issue** with the following structure:

```markdown
# Title: Brief description of the issue

**Type:** Bug / Feature Request / Improvement

**Status:** Open / In Progress / Resolved

## Description

A clear and concise description of the issue or feature request.

## Steps to Reproduce (for bugs)

1. Step one
2. Step two
3. ...

## Expected Behavior

What you expected to happen.

## Actual Behavior

What actually happened.

## Environment (if relevant)

- OS: [e.g., Ubuntu 22.04]
- Go version: [e.g., 1.23.0]
- calmailproc version: [e.g., commit hash or version]

## Proposed Solution (optional)

Your ideas on how to fix or implement this.

## Additional Context

Any other context, logs, or screenshots.
```

3. **Submit a pull request** to the `main` branch with your issue file

## Resolving Issues

When an issue is resolved:
- Update the **Status** field in the issue file to `Resolved`
- Add a section at the end documenting the resolution:
  ```markdown
  ## Resolution

  **Resolved in:** [commit hash or version]
  **Resolved by:** [contributor name/username]

  Description of how the issue was resolved.
  ```

## Why File-Based Issues?

- **Version Control**: Issues are tracked alongside code changes
- **Searchability**: Full-text search through all issues using standard tools
- **Documentation**: Issues serve as living documentation of the project
- **Offline Access**: Review and create issues without internet connectivity
- **Integration**: Issues can be referenced in commits and documentation
