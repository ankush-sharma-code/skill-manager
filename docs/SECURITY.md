# Security

The skill repository assumes a single user, and a single process tree, working in that user's home directory.

## Check then act

Publishing a skill and recovering `.stage` look at a path with `Lstat`, then create, rename, or remove it. Those steps are not one atomic operation against other processes. A second process can replace a directory with a symlink after the check and before the rename. This code does not call `openat`, `O_NOFOLLOW`, or `renameat`.

`processLock` in `internal/ports/repo` is held across `List`, `Get`, `Put`, `Delete`, `Recover`, and `Problems`. Goroutines in this process do not interleave a check with the action that follows it.

The daemon and the CLI are two processes. They share a lock file so one of them does not enter a repo call while the other is between `Lstat` and the rename or read that follows it. `processLock` does not cross that process boundary. A process that does not hold the lock file can still replace a path after the check.

`readFiles` learns a directory entry's type from `ReadDir`, then calls `ReadFile` on that name. Those are separate steps. A regular file can be replaced with a symlink in the gap, and `ReadFile` follows the replacement. The type check does not cover the bytes that are read.

## Directory mode

The repository creates its root and `.stage` with mode `0700`. `.stage/rejected` is also `0700`. Other users on the machine cannot write those directories. That is the boundary for a home directory owned by one person. Do not point the repository at a directory that another user can write. A root or `.stage` that already exists keeps the mode it already has.

`.trash` is created with mode `0755`. Other users can list it and open the skill trees moved there.
