package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestInstallWritesIdentityAndSoulUnderStateDir(t *testing.T) {
	initViperDefaults()

	stateDir := t.TempDir()
	workspaceDir := t.TempDir()

	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workspaceDir); err != nil {
		t.Fatalf("chdir workspace: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})

	cmd := newInstallCmd()
	cmd.SetArgs([]string{"--yes", stateDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install command failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(stateDir, "IDENTITY.md")); err != nil {
		t.Fatalf("IDENTITY.md should exist under state dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "SOUL.md")); err != nil {
		t.Fatalf("SOUL.md should exist under state dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "memory", "index.md")); err != nil {
		t.Fatalf("memory/index.md should exist under state dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspaceDir, "IDENTITY.md")); !os.IsNotExist(err) {
		t.Fatalf("IDENTITY.md should not be created in workspace root, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(workspaceDir, "SOUL.md")); !os.IsNotExist(err) {
		t.Fatalf("SOUL.md should not be created in workspace root, err=%v", err)
	}
}

func TestInstallUsesConfiguredStateDirWhenArgMissing(t *testing.T) {
	initViperDefaults()

	stateDir := filepath.Join(t.TempDir(), "configured-state")
	workspaceDir := t.TempDir()

	prevStateDir := viper.GetString("file_state_dir")
	viper.Set("file_state_dir", stateDir)
	t.Cleanup(func() {
		if prevStateDir == "" {
			viper.Set("file_state_dir", nil)
			return
		}
		viper.Set("file_state_dir", prevStateDir)
	})

	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workspaceDir); err != nil {
		t.Fatalf("chdir workspace: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})

	cmd := newInstallCmd()
	cmd.SetArgs([]string{"--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install command failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(stateDir, "IDENTITY.md")); err != nil {
		t.Fatalf("IDENTITY.md should exist under configured file_state_dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "SOUL.md")); err != nil {
		t.Fatalf("SOUL.md should exist under configured file_state_dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "memory", "index.md")); err != nil {
		t.Fatalf("memory/index.md should exist under configured file_state_dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspaceDir, "IDENTITY.md")); !os.IsNotExist(err) {
		t.Fatalf("IDENTITY.md should not be created in workspace root, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(workspaceDir, "SOUL.md")); !os.IsNotExist(err) {
		t.Fatalf("SOUL.md should not be created in workspace root, err=%v", err)
	}
}

func TestLoadIdentityTemplate(t *testing.T) {
	body, err := loadIdentityTemplate()
	if err != nil {
		t.Fatalf("loadIdentityTemplate() error = %v", err)
	}
	if body == "" {
		t.Fatalf("expected non-empty IDENTITY template")
	}
	if !strings.Contains(body, "# IDENTITY.md - Who Am I?") {
		t.Fatalf("IDENTITY template seems invalid")
	}
}

func TestLoadSoulTemplate(t *testing.T) {
	body, err := loadSoulTemplate()
	if err != nil {
		t.Fatalf("loadSoulTemplate() error = %v", err)
	}
	if body == "" {
		t.Fatalf("expected non-empty SOUL template")
	}
	if !strings.Contains(body, "# SOUL.md - Who You Are") {
		t.Fatalf("SOUL template seems invalid")
	}
}

func TestLoadToolsTemplate(t *testing.T) {
	body, err := loadToolsTemplate()
	if err != nil {
		t.Fatalf("loadToolsTemplate() error = %v", err)
	}
	if body == "" {
		t.Fatalf("expected non-empty TOOLS template")
	}
	if !strings.Contains(body, "# SCRIPTS.md - Local Scripts Notes") {
		t.Fatalf("TOOLS template seems invalid")
	}
}

func TestLoadTodoWIPTemplate(t *testing.T) {
	body, err := loadTodoWIPTemplate()
	if err != nil {
		t.Fatalf("loadTodoWIPTemplate() error = %v", err)
	}
	if body == "" {
		t.Fatalf("expected non-empty TODO.WIP template")
	}
	if !strings.Contains(body, "# TODO Work In Progress (WIP)") {
		t.Fatalf("TODO.WIP template seems invalid")
	}
}

func TestInstallCommandExposesYesFlag(t *testing.T) {
	cmd := newInstallCmd()
	flag := cmd.Flags().Lookup("yes")
	if flag == nil {
		t.Fatalf("expected --yes flag to exist")
	}
	if flag.Shorthand != "y" {
		t.Fatalf("expected --yes shorthand to be -y, got %q", flag.Shorthand)
	}
}

func TestLoadTodoDoneTemplate(t *testing.T) {
	body, err := loadTodoDoneTemplate()
	if err != nil {
		t.Fatalf("loadTodoDoneTemplate() error = %v", err)
	}
	if body == "" {
		t.Fatalf("expected non-empty TODO.DONE template")
	}
	if !strings.Contains(body, "# TODO Done") {
		t.Fatalf("TODO.DONE template seems invalid")
	}
}

func TestLoadContactsActiveTemplate(t *testing.T) {
	body, err := loadContactsActiveTemplate()
	if err != nil {
		t.Fatalf("loadContactsActiveTemplate() error = %v", err)
	}
	if body == "" {
		t.Fatalf("expected non-empty contacts ACTIVE template")
	}
	if !strings.Contains(body, "# Active Contacts") {
		t.Fatalf("contacts ACTIVE template seems invalid")
	}
}

func TestLoadContactsInactiveTemplate(t *testing.T) {
	body, err := loadContactsInactiveTemplate()
	if err != nil {
		t.Fatalf("loadContactsInactiveTemplate() error = %v", err)
	}
	if body == "" {
		t.Fatalf("expected non-empty contacts INACTIVE template")
	}
	if !strings.Contains(body, "# Inactive Contacts") {
		t.Fatalf("contacts INACTIVE template seems invalid")
	}
}

func TestLoadMemoryIndexTemplate(t *testing.T) {
	body, err := loadMemoryIndexTemplate()
	if err != nil {
		t.Fatalf("loadMemoryIndexTemplate() error = %v", err)
	}
	if body == "" {
		t.Fatalf("expected non-empty memory index template")
	}
	if !strings.Contains(body, "# Long-Term Memory") {
		t.Fatalf("memory index template seems invalid")
	}
}
