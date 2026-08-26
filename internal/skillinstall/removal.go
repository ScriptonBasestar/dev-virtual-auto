package skillinstall

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

func stageManagedRemoval(destination string, files []fileHash) (func() error, func() error, string, error) {
	stage, err := os.MkdirTemp(destination, ".dva-skill-remove-")
	if err != nil {
		return nil, nil, "", err
	}
	names := skillNames(skillBundle{files: files})
	type move struct{ source, staged string }
	moves := make([]move, 0, len(names))
	rollback := func() error {
		var first error
		for _, move := range slices.Backward(moves) {
			if err := os.Rename(move.staged, move.source); err != nil && first == nil {
				first = err
			}
		}
		if first == nil {
			if err := os.RemoveAll(stage); err != nil {
				first = err
			}
		}
		if err := syncDirectory(destination); err != nil && first == nil {
			first = err
		}
		return first
	}
	for _, name := range names {
		source := filepath.Join(destination, name)
		staged := filepath.Join(stage, name)
		if err := os.Rename(source, staged); err != nil {
			if rollbackErr := rollback(); rollbackErr != nil {
				return nil, nil, stage, fmt.Errorf("stage managed removal: %w (rollback failed: %v; recovery stage: %s)", err, rollbackErr, stage)
			}
			if errors.Is(err, os.ErrNotExist) {
				return nil, nil, "", fmt.Errorf("managed skill disappeared before removal: %s", source)
			}
			return nil, nil, "", err
		}
		moves = append(moves, move{source: source, staged: staged})
	}
	if err := syncDirectory(destination); err != nil {
		if rollbackErr := rollback(); rollbackErr != nil {
			return nil, nil, stage, fmt.Errorf("sync staged removal: %w (rollback failed: %v; recovery stage: %s)", err, rollbackErr, stage)
		}
		return nil, nil, "", err
	}
	finalize := func() error {
		if err := os.RemoveAll(stage); err != nil {
			return err
		}
		return syncDirectory(destination)
	}
	return rollback, finalize, stage, nil
}
