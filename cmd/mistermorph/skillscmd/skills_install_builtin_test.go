package skillscmd

import "testing"

func TestSkillsInstallCommandExposesYesFlagShorthand(t *testing.T) {
	cmd := NewSkillsInstallBuiltinCmd()
	flag := cmd.Flags().Lookup("yes")
	if flag == nil {
		t.Fatalf("expected --yes flag to exist")
	}
	if flag.Shorthand != "y" {
		t.Fatalf("expected --yes shorthand to be -y, got %q", flag.Shorthand)
	}
}
