package sh

import (
	"path/filepath"
	"testing"

	"github.com/james-orcales/golang_snacks/myers/snap"
)

func TestPushDir(t *testing.T) {
	before := WorkingDirectory()
	if filepath.Base(before) != "sh" {
		panic("Working directory must be the package directory")
	}
	popDir := PushDir("../../")
	current := filepath.Base(WorkingDirectory())
	if !snap.Init(`golang_snacks`).IsEqual(current) {
		t.Error("PushDir didn't change into the project root")
	}
	popDir()
	after := WorkingDirectory()
	if before != after {
		t.Error("PushDir didn't revert the original directory")
	}
}
