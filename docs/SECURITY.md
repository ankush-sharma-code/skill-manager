# Security

The skill repository assumes a single user, and a single process tree, working in that user's home directory.

## Check then act

Publishing a skill and recovering `.stage` look at a path with `Lstat`, then create, rename, or remove it. Those steps are not one atomic operation against other processes. A second process can replace a directory with a symlink after the check and before the rename. This code does not call `openat`, `O_NOFOLLOW`, or `renameat`.

`processLock` in `internal/ports/repo` is held across `List`, `Get`, `Put`, `Delete`, `Recover`, and `Problems`. Goroutines in this process do not interleave a check with the action that follows it. The lock does not apply to any other process.

## Directory mode

The repository creates its root and `.stage` with mode `0700`. Other users on the machine cannot write those directories. That is the boundary for a home directory owned by one person. Do not point the repository at a directory that another user can write. A root or `.stage` that already exists keeps the mode it already has.
