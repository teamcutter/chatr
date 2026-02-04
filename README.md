# Chatr

A package manager CLI for downloading, installing, and managing binary packages.

## Installation

```bash
curl -sL https://raw.githubusercontent.com/teamcutter/chatr/main/install.sh | sh
```

Add the chatr bin directory to your PATH:

```bash
export PATH="$HOME/.chatr/bin:$PATH"
```

Add this line to your shell configuration file (`~/.bashrc`, `~/.zshrc`, etc.) to make it permanent.

## Build from Source

### Prerequisites

- Go 1.25 or later

### Build

```bash
mkdir -p ~/.chatr/bin && go build -o ~/.chatr/bin/chatr ./cmd/chatr
```

## Usage

### Install a package

```bash
~/ chatr install hello
Downloading hello 100% |█████████████████████████████████████████████| (53/53 kB, 540 kB/s)

✓ hello@2.12.2
  cache: /Users/user/.chatr/cache/hello/2.12.2
  path: /Users/user/.chatr/packages/hello/2.12.2

~/ hello
Hello, world!
```

## Commands

### install

Install one or more packages.

```bash
chatr install <name>...
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--version` | `-v` | `latest` | Package version |
| `--sha256` | | | Expected SHA256 checksum |

### remove

Remove one or more installed packages.

```bash
chatr remove <name>...
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--version` | `-v` | `latest` | Package version to remove |

### list

List all installed packages.

```bash
chatr list
```

### search

Search for packages in the registry.

```bash
chatr search <query>
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--show` | `-s` | `50` | Number of results to display |

### clear

Clear the packages cache.

```bash
chatr clear
```

### version

Print the version of chatr.

```bash
chatr version
```

### new

Update chatr to the newest version.

```bash
chatr new
```

## License

Apache License 2.0
