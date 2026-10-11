# Security policy

## Reporting a vulnerability

Please **do not** report security problems in public issues.

Report them privately through GitHub: open the repository's **Security** tab and choose **Report a vulnerability** ([direct link](https://github.com/deckplane/deckplane-cli/security/advisories/new)).

Include what you found, the steps to reproduce it and the CLI version (`deckplane version`). We will confirm receipt within three working days and keep you updated until it is fixed. Please give us a reasonable time to release a fix before disclosing the issue.

## Supported versions

Security fixes are released for the latest version only. Update by running the installer again, or with `go install github.com/deckplane/deckplane-cli@latest`.

## How the CLI handles secrets

- License keys, tokens and generated passwords are stored only in the install folder's `.env`, readable by its owner only (mode 0600, folder 0700).
- Secrets are passed to Docker on standard input, never as command-line arguments, and are masked in the CLI's output.
- Registry credentials are removed from Docker's credential store (`docker logout`) after every install or update.
- The installers verify each download against the release's `checksums.txt`.
