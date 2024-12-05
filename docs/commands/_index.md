---
title: Commands
weight: 2
---

The [`super` command](super.md) is used to execute command-line queries on
inputs from files, HTTP URLs, or [S3](../integrations/amazon-s3.md).

The [`super db` sub-commands](super-db.md) are for creating, configuring, ingesting
into, querying, and orchestrating SuperDB data lakes. These sub-commands are
organized into further subcommands like the familiar command patterns
of `docker` or `kubectl`.

All operations with these commands utilize the [super data model](../formats/_index.md)
and provide querying via [SuperSQL](../language/_index.md).

Built-in help for `super` and all sub-commands is always accessible with the `-h` flag.
