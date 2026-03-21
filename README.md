# ghostty-shader-manager

A CLI utility for managing [Ghostty](https://ghostty.org) shaders. Enable, disable, and toggle shaders without hand-editing config files — Ghostty reloads automatically after every change.

## How it works

Ghostty supports a `custom-shader` directive in its config. `ghostty-shader-manager` owns a dedicated include file (`~/.config/ghostty/ghostty-shaders`) where it writes those directives, commenting out disabled shaders. You register shaders once in a config YAML; from then on, the CLI and interactive UI handle the rest.

## Prerequisites

- [fzf](https://github.com/junegunn/fzf) (for interactive mode)
- Ghostty config must include the managed file:
  ```
  config-file = ~/.config/ghostty/ghostty-shaders
  ```

## Installation

```sh
go install github.com/brianmargolis/ghostty-shades-manager@latest
```

Or build from source:

```sh
go build -o ghostty-shader-manager .
```

## Configuration

Create `~/.config/ghostty-shader-manager/ghostty-shader-manager.config.yaml`:

```yaml
shaders:
  - path: ~/.config/ghostty/shaders/bettercrt.glsl
    name: crt
  - path: ~/.config/ghostty/shaders/bloom.glsl
    name: bloom
```

Each entry needs a `path` (tilde-expanded) and a `name` used to reference the shader in CLI commands.

Then run `sync` once to initialize the managed file:

```sh
ghostty-shader-manager sync
```

## Usage

```
ghostty-shader-manager              # interactive mode (fzf)
ghostty-shader-manager sync         # sync config → shaders file
ghostty-shader-manager list         # list registered shaders
ghostty-shader-manager status       # show enabled/disabled state
ghostty-shader-manager on <name>    # enable a shader
ghostty-shader-manager off <name>   # disable a shader
```

### Interactive mode

Running `ghostty-shader-manager` with no arguments opens an fzf picker showing all registered shaders with their current state (`[on]` / `[off]`). Use **Tab** to mark shaders to toggle, then **Enter** to apply. **Ctrl-C** or **Esc** exits with no changes.

### Reload behavior

Every write operation sends `SIGUSR2` to Ghostty, triggering a live config reload. Changes take effect immediately.
