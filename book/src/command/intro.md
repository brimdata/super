# Command Usage

`super` is the command-line tool for interacting with and managing SuperDB
The command is organized as a hierarchy of sub-commands similar to
[`docker`](https://docs.docker.com/engine/reference/commandline/cli/)
or [`kubectl`](https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands).

SuperDB does not have a [REPL](https://en.wikipedia.org/wiki/Read%E2%80%93eval%E2%80%93print_loop).
Instead, your shell is your REPL and the `super` command lets you
* run SuperSQL queries attached to or detached from
a database)
* compile and inspect query plans
* run a SuperDB service endpoint,
* or access some built-in dev tooling when want to dive deep.
