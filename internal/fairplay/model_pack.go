package fairplay

import (
	"fmt"
	"os"
	"path/filepath"
)

// loadCurrentSINFModel returns the current SINF model pack and the name that
// identifies it.
//
// The embedded pack covers the FairPlay generation that the upstream tool was
// built against. A SINF that was produced by a different generation is not
// decoded by it, so an external pack can be supplied instead.
func loadCurrentSINFModel(path string) ([]byte, string, error) {
	if path == "" {
		if digest(currentSINFModelPack) != currentSINFModelSHA256 {
			return nil, "", fmt.Errorf("embedded current SINF model is corrupt")
		}

		return currentSINFModelPack, currentSINFModelName, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read current SINF model: %w", err)
	}

	if _, err := parseCurrentSINFModel(data, ""); err != nil {
		return nil, "", fmt.Errorf("current SINF model %s: %w", filepath.Base(path), err)
	}

	return data, filepath.Base(path), nil
}
