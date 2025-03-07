# GitHub Fork Cleanup

🧹 A GitHub CLI extension that helps you clean up your forked repositories through an interactive interface.

## Features

- 🔍 Detects and warns about forks with open pull requests
- 📅 Shows last update time for each fork
- ✨ Interactive yes/no prompts for each repository
- 🔒 Safe deletion process using GitHub CLI

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

## Screenshots

![Demo](demo.gif) _(Add a demo.gif to showcase the extension in action)_

## Contributing

Contributions are welcome! Feel free to open issues or submit pull requests.

## License

MIT License - feel free to use and modify this code for your own projects.
