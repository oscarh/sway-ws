# sway-ws

Two-level workspace navigation for sway: workspaces are grouped into
projects by name, and numbered within each project.

## Naming

Workspaces are named `<project>:<n>[:<label>]`, e.g. `home:1`,
`home:2:fw`. The number is the slot that `focus N` and `move N` address;
the label is only for reading. A project may also hold unnumbered
workspaces (`home:notes`).

A name without a colon (`slack`, or sway's default `1`) belongs to the
`main` project, so workspaces created outside the convention stay
reachable. Rename them with `sway-ws rename` to move them into a project.

The current project is always the one of the focused workspace.

## Commands

| Command          | Action                                                         |
|------------------|----------------------------------------------------------------|
| `focus N`        | Focus slot N of the current project (created if missing)       |
| `move N`         | Move the focused window to slot N of the current project       |
| `toggle`         | Previous workspace within the current project                  |
| `project-toggle` | Previous project, landing on the workspace last used there     |
| `pick`           | wmenu over all projects and workspaces; focus the choice       |
| `pick-move`      | As `pick`, but move the focused window there                   |
| `rename`         | Rename the focused workspace (Tab copies the current name)     |
| `label`          | Change only the label of the focused workspace (`-` clears)    |
| `nth N`          | Focus the N-th workspace of the current project (bar clicks)   |
| `other N`        | Go to the N-th other project, alphabetically (bar clicks)      |
| `waybar`         | Waybar module: the current project, streamed                   |
| `waybar-ws N`    | Waybar module: the N-th workspace of the current project       |
| `waybar-more N`  | Waybar module: number of workspaces beyond the first N         |
| `waybar-other N` | Waybar module: the N-th other project                          |
| `version`        | Print the version                                              |

### Picker

What is selected or typed in `pick` / `pick-move` resolves as:

- an existing workspace: that workspace
- an existing project: the workspace last used in it
- `proj:` (trailing colon): the same, or `proj:1` for a new project
- any other bare name: a new workspace in the current project, in its
  lowest free slot (`home:3:name`)
- anything else with a colon: created as typed

### Labels

`label` keeps the project and number and only changes the label:
`home:2:fw` becomes `home:2:db`, or `home:2` when cleared with `-`. A
cleared unnumbered workspace (`home:notes`) gets the lowest free slot.
Characters that sway's command parser treats specially are dropped.

## Several outputs

A project can be on several outputs at once, with one of its workspaces on
each. Every command records which workspaces of the current project are on
screen. Entering another project (`project-toggle`, the picker, a click on
another project) brings back the workspaces it had on screen before
focusing the target, in a single sway command so the outputs switch
together. Within a project, sway's own output handling applies.

## State

There is no daemon. Each command records the focused workspace, acts, and
writes per-project history (last and previous workspace, the workspaces on
screen, previous project)
to `$XDG_RUNTIME_DIR/sway-ws/<hash>.json`. The file is keyed on `$SWAYSOCK`,
so nested sway sessions are kept apart, and it is locked while a command
runs. Switches made outside sway-ws, such as mouse clicks, are picked up by
the next sway-ws command. The history is gone after logout, like the
workspaces it describes.

## Sway config

```
set $sp ~/.local/bin/sway-ws
bindsym $mod+1 exec $sp focus 1           # ... through 9
bindsym $mod+Shift+1 exec $sp move 1      # ... through 9
bindsym $mod+Tab exec $sp toggle
bindsym $mod+Shift+Tab exec $sp project-toggle
bindsym $mod+t exec $sp pick
bindsym $mod+Shift+t exec $sp pick-move
bindsym $mod+r exec $sp rename
bindsym $mod+n exec $sp label
```

## Waybar

The bar is one module for the project, one per workspace position, a
`+N` overflow module and one per other project, so that each can be
clicked and styled. Only the `waybar` module keeps running. On every
workspace change it sends its own waybar `SIGRTMIN+8`, and the other
modules, which run once per signal, refresh. With several bars in one
waybar process, a lock file makes only one module send the signal.

```jsonc
"custom/sway-ws": {
    "exec": "$HOME/.local/bin/sway-ws waybar",
    "return-type": "json",
    "restart-interval": 1,
    "on-click": "$HOME/.local/bin/sway-ws pick"
},
"custom/sway-ws-1": {
    "exec": "$HOME/.local/bin/sway-ws waybar-ws 1",
    "return-type": "json",
    "interval": "once",
    "signal": 8,
    "on-click": "$HOME/.local/bin/sway-ws nth 1"
},
// ... one per position, then waybar-more and waybar-other N the same way
```

No other waybar module may use signal 8. A module with nothing to show
prints empty text, which hides it.

CSS classes set by the modules:

| Module  | Classes                                                 |
|---------|---------------------------------------------------------|
| project | `project`                                               |
| ws      | `ws`, plus one of `focused`, `visible`, `urgent`        |
| more    | `ws more`, plus `urgent` if a hidden one is urgent      |
| other   | `other`, plus `urgent` or `visible` (on another output) |

A workspace is urgent when sway marks it so: a native Wayland window that
requests activation (sway's default `focus_on_window_activation urgent`),
or an X11 window with the urgency hint.

## Build

```
make test
make install    # builds and installs to ~/.local/bin
```

The version is taken from `git describe --tags --always --dirty`.
