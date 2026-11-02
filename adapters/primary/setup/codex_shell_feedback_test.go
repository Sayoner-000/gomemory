package setup

import "testing"

func TestHCD014CodexShellInstallationVersion(t *testing.T) {
	if !CodexShellFeedbackVersionSupported("codex-cli 0.166.0\n") {
		t.Fatal("observed version not accepted")
	}
	for _, v := range []string{"", "codex-cli 0.166.1", "codex-cli 0.167.0", "codex-cli 0.165.0"} {
		if CodexShellFeedbackVersionSupported(v) {
			t.Fatalf("unobserved version accepted: %q", v)
		}
	}
	t.Setenv("PATH", t.TempDir())
	if CodexShellFeedbackSupported() {
		t.Fatal("missing executable accepted")
	}
	if CodexShellFeedbackHook.Matcher != "^Bash$" || CodexShellFeedbackHook.Sub != "tool-output codex-shell" {
		t.Fatalf("shell hook expanded outside proven surface: %+v", CodexShellFeedbackHook)
	}
}
