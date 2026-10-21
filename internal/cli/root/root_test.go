package root

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootHelpListsRegisteredCommands(t *testing.T) {
	cmd := New()
	sub := New()
	sub.Use = "sub"
	sub.Short = "a subcommand"
	cmd.AddCommand(sub)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	for _, want := range []string{"ppautil", "sub", "a subcommand"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help output missing %q:\n%s", want, out.String())
		}
	}
}
