package fairplay

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCurrentSINFModel(t *testing.T) {
	pack, name, err := loadCurrentSINFModel("")
	if err != nil {
		t.Fatalf("load embedded model: %v", err)
	}

	if name != currentSINFModelName {
		t.Fatalf("embedded model name is %q, want %q", name, currentSINFModelName)
	}

	if digest(pack) != currentSINFModelSHA256 {
		t.Fatal("embedded model digest mismatch")
	}
}

func TestLoadExternalCurrentSINFModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom-model.bin")
	if err := os.WriteFile(path, currentSINFModelPack, 0o600); err != nil {
		t.Fatal(err)
	}

	pack, name, err := loadCurrentSINFModel(path)
	if err != nil {
		t.Fatalf("load external model: %v", err)
	}

	if name != "custom-model.bin" {
		t.Fatalf("external model name is %q, want %q", name, "custom-model.bin")
	}

	if digest(pack) != currentSINFModelSHA256 {
		t.Fatal("external model digest mismatch")
	}
}

func TestLoadExternalCurrentSINFModelRejectsInvalidPack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken-model.bin")
	if err := os.WriteFile(path, []byte("not a model pack"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, _, err := loadCurrentSINFModel(path); err == nil {
		t.Fatal("accepted an invalid model pack")
	}
}
