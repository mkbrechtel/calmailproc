# Design Overview

## Modules

Separate modules for parsing mails and storing the calendar events in the database.

- **parser**: Responsible for parsing incoming emails to extract relevant information about calendar events.
- **storage**: Responsible for storing calendar events in the database. This module just implements a storage interface.
- **storage/vdir**: Implements a storage interface for storing calendar events in the [vdir format](https://vdirsyncer.readthedocs.io/en/stable/vdir.html)
- **processor**: Responsible for processing the parsed calendar events and storing them in the database.

## Event Storage

The calendar events come via mail as text/calendar attachments. We implement a storage interface for storing calendar events. We use the UID as the primary key for the events. The storage module provides methods for creating, updating, and deleting events.

