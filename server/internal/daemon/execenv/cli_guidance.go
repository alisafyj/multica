package execenv

// BuildMulticaCLIInvocationGuidance returns the shell-specific rule for
// invoking the daemon-owned CLI. Windows keeps its existing command examples:
// the privacy guard rejects $env access, so this guidance is POSIX-only.
func BuildMulticaCLIInvocationGuidance(goos string) string {
	if goos == "windows" {
		return ""
	}
	return "On POSIX shells, run every `multica ...` example in these instructions as `\"$MULTICA_CLI\" ...`. The daemon-provided value selects the daemon's exact binary. If `MULTICA_CLI` is missing or empty, stop running Multica platform commands and report that the runtime configuration is missing; do not search for credentials or modify PATH.\n\n"
}
