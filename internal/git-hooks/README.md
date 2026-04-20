# git-hooks

Git hook integration for LP. This package provides a `Listener` implementation
that installs lightweight shim scripts into a Git repository's `.git/hooks/`
directory. When Git invokes a hook (e.g. `pre-commit`, `post-commit`,
`pre-push`), the shim script sends a request to a local HTTP server managed
by this listener, which in turn emits an LP `Event` to trigger a pipeline.

## How it works

1. On `Start`, a tiny HTTP server binds to a random localhost port.
2. For each configured hook name, a shim script is written to
   `<repo>/.git/hooks/<hook>`. The script `curl`s the local server.
3. When Git fires the hook, the shim script hits the HTTP server, which
   sends an `Event` on the LP event channel.
4. On `Stop`, the shim scripts are removed and the HTTP server shuts down.

## Configuration (YAML)

```yaml
listeners:
  - type: git_hook
    name: my-git-hooks
    pipeline: lint
    config:
      repo: /path/to/git/repo
      hooks:
        - pre-commit
        - post-commit
        - pre-push
```

### Config keys

| Key     | Required | Description                                         |
|---------|----------|-----------------------------------------------------|
| `repo`  | yes      | Absolute path to the Git repository root             |
| `hooks` | yes      | List of Git hook names to install shims for          |

## Backup behaviour

If an existing hook script is found, it is renamed to `<hook>.lp-backup`
before the shim is installed. On `Stop`, the backup is restored.

