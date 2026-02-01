# Chatr

A package manager CLI for downloading, installing, and managing binary packages.

## Setup

### Prerequisites

- Go 1.25 or later

### Build

```bash
mkdir -p ~/.chatr/bin && go build -o ~/.chatr/bin/chatr ./cmd/chatr

```

### Install

Add the chatr bin directory to your PATH:

```bash
export PATH="$HOME/.chatr/bin:$PATH"
```

Add this line to your shell configuration file (`~/.bashrc`, `~/.zshrc`, etc.) to make it permanent.

## Usage

```bash
# Install a package
chatr install jq@https://github.com/jqlang/jq/releases/download/jq-1.7.1/jq-linux64.tar.gz

# Install with version and checksum verification
chatr install jq@https://... --version 1.7.1 --sha256 <checksum>

# List installed packages
chatr list

# Uninstall a package
chatr uninstall jq
```

## License

Apache License 2.0
