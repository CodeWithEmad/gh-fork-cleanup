# GitHub Fork Cleanup

🧹 A GitHub CLI extension that helps you clean up your forked repositories through an interactive interface.

![demo](https://github.com/user-attachments/assets/d92ebcb2-774d-464a-b04c-aa0176bd8c90)

## Features

- 🔍 Detects and warns about forks with open pull requests
- 🔄 Shows commit comparison with parent repository
- 📦 Identifies archived repositories
- 📅 Shows last update time for each fork
- ✨ Interactive yes/no prompts for each repository
- 🔒 Safe deletion process using GitHub CLI
- 🌐 Open fork URLs directly in your browser

## Installation

First, make sure you have the [GitHub CLI](https://cli.github.com/) installed and authenticated.

```bash
# Install the extension
gh extension install CodeWithEmad/gh-fork-cleanup
```

> [!IMPORTANT]
> Your GitHub token must have the `delete_repo` scope to delete forks. If you're using GitHub CLI's
> built-in authentication, ensure this scope is included. To add the scope to your existing token, run:
>
> ```bash
> gh auth refresh -h github.com -s delete_repo
> ```

## Usage

Simply run:

```bash
gh fork-cleanup [--force|-f] [--skip-confirmation|-s]
```

Options:

- `--force, -f`: It will automatically delete all forks. Be careful when using this option.
- `--skip-confirmation, -s`: Skip the extra confirmation step for forks with open pull requests.

The extension will:

1. Show a loading spinner while fetching your forks
2. Check for any open pull requests from your forks
3. List all your forked repositories with their last update times
4. Warn you about forks that have open pull requests
5. Ask if you want to delete each fork
6. Process your choice (delete or skip)

## Development

1. Fork and clone the repository

    ```bash
    gh repo fork CodeWithEmad/gh-fork-cleanup # or just use Github website
    git@github.com:YOUR_USERNAME/gh-fork-cleanup.git
    ```

2. Make your changes to the code

3. Build the executable

    ```bash
    go build -o gh-fork-cleanup
    ```

and enjoy hacking!

## Contributing

Contributions are welcome! Feel free to open issues or submit pull requests.

## License

MIT License - feel free to use and modify this code for your own projects.
