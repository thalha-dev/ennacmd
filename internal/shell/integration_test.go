package shell

import (
	"os"
	"strings"
	"testing"
)

func TestIntegrationScriptUsesCaptureMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		kind     Kind
		contains []string
	}{
		{
			name: "zsh",
			kind: Zsh,
			contains: []string{
				"--capture",
				"print -z --",
				"__ennacmd_resolve_binary",
			},
		},
		{
			name: "bash",
			kind: Bash,
			contains: []string{
				"--capture",
				"bind -x",
				"printf '%s\\n'",
				"type -P ennacmd",
			},
		},
		{
			name: "fish",
			kind: Fish,
			contains: []string{
				"--capture",
				"commandline --replace",
				"printf '%s\\n'",
				"type -p ennacmd",
			},
		},
		{
			name: "powershell",
			kind: PowerShell,
			contains: []string{
				"--capture",
				"PSConsoleReadLine",
				"Get-EnnacmdBinary",
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			script, err := IntegrationScript(testCase.kind)
			if err != nil {
				t.Fatalf("IntegrationScript(%q) returned error: %v", testCase.kind, err)
			}

			for _, expected := range testCase.contains {
				if !strings.Contains(script, expected) {
					t.Fatalf("IntegrationScript(%q) did not contain %q\nscript:\n%s", testCase.kind, expected, script)
				}
			}

			if strings.Contains(script, "%!") {
				t.Fatalf("IntegrationScript(%q) contained fmt formatting corruption\nscript:\n%s", testCase.kind, script)
			}
		})
	}
}

func TestIntegrationScriptDoesNotEmbedCurrentExecutablePath(t *testing.T) {
	t.Parallel()

	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable returned error: %v", err)
	}

	for _, kind := range []Kind{Zsh, Bash, Fish, PowerShell} {
		kind := kind
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()

			script, err := IntegrationScript(kind)
			if err != nil {
				t.Fatalf("IntegrationScript(%q) returned error: %v", kind, err)
			}

			if strings.Contains(script, executable) {
				t.Fatalf("IntegrationScript(%q) unexpectedly embedded executable path %q\nscript:\n%s", kind, executable, script)
			}
		})
	}
}

func TestIntegrationScriptWithFallbackEmbedsFallbackPath(t *testing.T) {
	t.Parallel()

	const fallback = "/tmp/ennacmd/bin/ennacmd"
	script, err := integrationScript(Zsh, fallback)
	if err != nil {
		t.Fatalf("integrationScript returned error: %v", err)
	}

	if !strings.Contains(script, fallback) {
		t.Fatalf("integrationScript should include fallback path %q\nscript:\n%s", fallback, script)
	}
	if !strings.Contains(script, "whence -p ennacmd") {
		t.Fatalf("integrationScript should prefer PATH resolution before fallback\nscript:\n%s", script)
	}
}

func TestMergeManagedBlockAddsManagedBlock(t *testing.T) {
	t.Parallel()

	updated := mergeManagedBlock("export TEST=1\n", "echo test")
	if !strings.Contains(updated, integrationStartMarker) {
		t.Fatalf("managed block start marker missing: %q", updated)
	}
	if !strings.Contains(updated, integrationEndMarker) {
		t.Fatalf("managed block end marker missing: %q", updated)
	}
	if !strings.Contains(updated, "echo test") {
		t.Fatalf("managed block body missing: %q", updated)
	}
}

func TestMergeManagedBlockReplacesExistingBlock(t *testing.T) {
	t.Parallel()

	existing := strings.Join([]string{
		"export TEST=1",
		integrationStartMarker,
		"old body",
		integrationEndMarker,
		"export OTHER=1",
	}, "\n")

	updated := mergeManagedBlock(existing, "new body")
	if strings.Count(updated, integrationStartMarker) != 1 {
		t.Fatalf("expected one start marker, got %d\n%s", strings.Count(updated, integrationStartMarker), updated)
	}
	if strings.Count(updated, integrationEndMarker) != 1 {
		t.Fatalf("expected one end marker, got %d\n%s", strings.Count(updated, integrationEndMarker), updated)
	}
	if strings.Contains(updated, "old body") {
		t.Fatalf("old body should have been replaced: %q", updated)
	}
	if !strings.Contains(updated, "new body") {
		t.Fatalf("new body missing: %q", updated)
	}
	if !strings.Contains(updated, "export OTHER=1") {
		t.Fatalf("remainder after managed block should be preserved: %q", updated)
	}
}
