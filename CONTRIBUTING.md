# Contributing To `git-different`

Thank you for investing your time in contributing to this project!

`git-different` is a fork of [`diffnav`](https://github.com/dlvhdr/diffnav) by
[@dlvhdr](https://github.com/dlvhdr). If you're looking to contribute
upstream-compatible changes, consider sending them to the original project.

## The Critical Rule

- The most important rule: you must understand your code. If you can't explain what your changes do and how they interact with the greater system without the aid of AI tools, do not contribute to this project.
- The second most important rule: when you submit a PR you must be willing to address comments and maintain this code. Do not submit drive-by PRs that solve your own issue without the willingness to iterate on it. Keep these in your own fork.
- Using AI to write code is fine. You can gain understanding by interrogating an agent with access to the codebase until you grasp all edge cases and effects of your changes. What's not fine is submitting agent-generated slop without that understanding. Be sure to read the [AI Usage Policy](AI_POLICY.md).

## AI Usage

The project has strict rules for AI usage. Please see the [AI Usage Policy](AI_POLICY.md). This is very important.

## Quick Guide

### I Have an Idea for a Feature

First search through both issues and discussions to see if your feature has already been requested. Otherwise, open an issue at [https://github.com/WhiteAbeLincoln/git-different/issues](https://github.com/WhiteAbeLincoln/git-different/issues).

### I've Implemented a Feature

- If there is an issue for the feature, open a pull request straight away.
- If there is no issue, open a discussion and link to your branch.
- If you want to live dangerously, open a pull request and hope for the best.

## Working on the Code

### Installing Required Tooling

The project uses [Nix flakes](https://nixos.wiki/wiki/Flakes) to manage its development environment.

- Clone this repo

```sh
git clone git@github.com:WhiteAbeLincoln/git-different.git && cd git-different
```

- Enter the dev shell (requires `nix` with flakes enabled):

```sh
nix develop
```

This will drop you in a shell with `go`, `gopls`, `golangci-lint`, `betteralign`, and other tools required to build and lint the project.

- _(Optional)_ Set up `direnv` so the dev shell loads automatically when you `cd` into the repo. See [nix-direnv](https://github.com/nix-community/nix-direnv).

### Navigating the Codebase

To navigate the codebase with confidence, familiarize yourself with:

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — the TUI framework
- [The Elm architecture](https://guide.elm-lang.org/architecture/)
- [`go-gitdiff`](https://github.com/bluekeyes/go-gitdiff) — unified diff parser

### Debugging

- Write to the log using Charm's `log` package
- Run `git-different` in debug mode by setting `DEBUG=true`; logs are appended to `./debug.log` in the current working directory
- Tail them with `tail -f debug.log` in another terminal

```go
import "charm.land/log/v2"

// more code...

log.Debug("some message", "someVariable", someVariable)
```
