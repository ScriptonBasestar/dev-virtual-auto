package skillinstall

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/skillclaim"
)

const dvaClaimProducer = "dva"

func newClaimOperationID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func claimDestinations(target destination, bundle skillBundle) ([]string, error) {
	names := skillNames(bundle)
	result := make([]string, 0, len(names))
	for _, name := range names {
		canonical, err := skillclaim.CanonicalDestination(claimDestination(target, name))
		if err != nil {
			return nil, err
		}
		result = append(result, canonical)
	}
	sort.Strings(result)
	return result, nil
}

func projectedClaims(target destination, scope Scope, runtimes []Runtime, bundle skillBundle, state, operationID string) ([]skillclaim.Claim, error) {
	consumers := make([]string, len(runtimes))
	for index, runtime := range runtimes {
		consumers[index] = string(runtime)
	}
	sort.Strings(consumers)
	claims := make([]skillclaim.Claim, 0, len(skillNames(bundle)))
	for _, installedName := range skillNames(bundle) {
		logicalName := strings.TrimSuffix(installedName, ".md")
		kind := skillclaim.KindDirectory
		var files []skillclaim.FileHash
		for _, file := range bundle.files {
			if strings.Contains(file.Path, "/") {
				prefix := logicalName + "/"
				if relative, found := strings.CutPrefix(file.Path, prefix); found {
					files = append(files, skillclaim.FileHash{Path: relative, SHA: file.SHA})
				}
			} else if file.Path == installedName {
				kind = skillclaim.KindFile
				files = append(files, skillclaim.FileHash{Path: ".", SHA: file.SHA})
			}
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("no source files found for claimed skill %s", installedName)
		}
		sourceDigest, err := skillclaim.ManifestDigest(files)
		if err != nil {
			return nil, err
		}
		canonical, err := skillclaim.CanonicalDestination(filepath.Join(target.path, installedName))
		if err != nil {
			return nil, err
		}
		claim := skillclaim.Claim{
			Schema: skillclaim.Schema, Name: logicalName, Kind: kind, State: state,
			OperationID: operationID, Generation: 1, Destination: canonical,
			Producer: dvaClaimProducer, Format: targetReceiptFormat(target), Scope: string(scope),
			Consumers: consumers, SourceDigest: sourceDigest, Files: files,
		}
		if err := skillclaim.Validate(claim, canonical); err != nil {
			return nil, err
		}
		claims = append(claims, claim)
	}
	sort.Slice(claims, func(i, j int) bool { return claims[i].Destination < claims[j].Destination })
	return claims, nil
}

func readLockedClaims(store *skillclaim.LockedStore, expected []skillclaim.Claim) ([]skillclaim.Claim, error) {
	claims := make([]skillclaim.Claim, 0, len(expected))
	for _, projection := range expected {
		claim, found, err := store.Read(projection.Destination)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("DVA claim is missing for %s", projection.Destination)
		}
		claims = append(claims, claim)
	}
	return claims, nil
}

func ensureClaimsAbsent(store *skillclaim.LockedStore, expected []skillclaim.Claim) error {
	for _, projection := range expected {
		claim, found, err := store.Read(projection.Destination)
		if err != nil {
			return err
		}
		if found {
			if claim.Producer != dvaClaimProducer {
				return fmt.Errorf("refusing skill claim at %s: owned by producer %q", projection.Destination, claim.Producer)
			}
			return fmt.Errorf("recovery-required: DVA claim exists without matching active receipt at %s", projection.Destination)
		}
	}
	return nil
}

func ensureClaimsAbsentUnlocked(root string, expected []skillclaim.Claim) error {
	for _, projection := range expected {
		claim, found, err := skillclaim.Read(root, projection.Destination)
		if err != nil {
			return err
		}
		if found {
			if claim.Producer != dvaClaimProducer {
				return fmt.Errorf("refusing skill claim at %s: owned by producer %q", projection.Destination, claim.Producer)
			}
			return fmt.Errorf("recovery-required: orphan DVA claim at %s", projection.Destination)
		}
	}
	return nil
}

func verifyClaimsUnlocked(root string, expected []skillclaim.Claim) error {
	actual := make([]skillclaim.Claim, 0, len(expected))
	for _, projection := range expected {
		claim, found, err := skillclaim.Read(root, projection.Destination)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("recovery-required: DVA claim is missing for %s", projection.Destination)
		}
		actual = append(actual, claim)
	}
	return verifyActiveClaims(actual, expected)
}

func verifyActiveClaims(actual, expected []skillclaim.Claim) error {
	if len(actual) != len(expected) {
		return errors.New("DVA claim count differs from receipt")
	}
	for index := range expected {
		left, right := actual[index], expected[index]
		if left.State != skillclaim.StateActive {
			return fmt.Errorf("recovery-required: DVA claim for %s is %s", left.Destination, left.State)
		}
		if left.Name != right.Name || left.Kind != right.Kind || left.Destination != right.Destination || left.Producer != right.Producer || left.Format != right.Format || left.Scope != right.Scope || left.SourceDigest != right.SourceDigest || !equalClaimStrings(left.Consumers, right.Consumers) || !equalClaimFiles(left.Files, right.Files) {
			return fmt.Errorf("recovery-required: DVA claim for %s differs from receipt", left.Destination)
		}
	}
	return nil
}

func equalClaimStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalClaimFiles(left, right []skillclaim.FileHash) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func reserveClaims(store *skillclaim.LockedStore, claims []skillclaim.Claim) ([]skillclaim.Claim, error) {
	reserved := make([]skillclaim.Claim, 0, len(claims))
	for _, claim := range claims {
		claim.State = skillclaim.StateReserved
		claim.Generation = 1
		if err := store.Reserve(claim); err != nil {
			return reserved, err
		}
		reserved = append(reserved, claim)
	}
	return reserved, nil
}

func activateReservedClaims(store *skillclaim.LockedStore, reserved []skillclaim.Claim) ([]skillclaim.Claim, error) {
	active := make([]skillclaim.Claim, 0, len(reserved))
	for _, claim := range reserved {
		previous, err := skillclaim.Digest(claim)
		if err != nil {
			return active, err
		}
		claim.State = skillclaim.StateActive
		claim.Generation++
		if err := store.CompareAndSwap(claim, claim.Generation-1, previous); err != nil {
			return active, err
		}
		active = append(active, claim)
	}
	return active, nil
}

func transitionActiveClaims(store *skillclaim.LockedStore, current, desired []skillclaim.Claim, state, operationID string) ([]skillclaim.Claim, error) {
	if len(current) != len(desired) {
		return nil, errors.New("claim transition count mismatch")
	}
	transitioned := make([]skillclaim.Claim, 0, len(current))
	for index := range current {
		next := current[index]
		if state == skillclaim.StateUpdating {
			next.Consumers = append([]string(nil), desired[index].Consumers...)
			next.SourceDigest = desired[index].SourceDigest
			next.Files = append([]skillclaim.FileHash(nil), desired[index].Files...)
		}
		next.State = state
		next.OperationID = operationID
		next.Generation++
		previous, err := skillclaim.Digest(current[index])
		if err != nil {
			return transitioned, err
		}
		if err := store.CompareAndSwap(next, current[index].Generation, previous); err != nil {
			return transitioned, err
		}
		transitioned = append(transitioned, next)
	}
	return transitioned, nil
}

func activateUpdatedClaims(store *skillclaim.LockedStore, updating []skillclaim.Claim) error {
	for _, claim := range updating {
		previous, err := skillclaim.Digest(claim)
		if err != nil {
			return err
		}
		claim.State = skillclaim.StateActive
		claim.Generation++
		if err := store.CompareAndSwap(claim, claim.Generation-1, previous); err != nil {
			return err
		}
	}
	return nil
}

func removeTransitionedClaims(store *skillclaim.LockedStore, claims []skillclaim.Claim) error {
	for _, claim := range claims {
		previous, err := skillclaim.Digest(claim)
		if err != nil {
			return err
		}
		if err := store.Remove(claim.Destination, claim.Producer, claim.OperationID, claim.Generation, previous); err != nil {
			return err
		}
	}
	return nil
}

func rollbackClaimsToAbsent(store *skillclaim.LockedStore, expected []skillclaim.Claim) error {
	for _, projection := range expected {
		current, found, err := store.Read(projection.Destination)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		if current.Producer != dvaClaimProducer {
			return fmt.Errorf("cannot roll back foreign claim at %s", current.Destination)
		}
		if current.State == skillclaim.StateReserved {
			previous, err := skillclaim.Digest(current)
			if err != nil {
				return err
			}
			active := current
			active.State = skillclaim.StateActive
			active.Generation++
			if err := store.CompareAndSwap(active, current.Generation, previous); err != nil {
				return err
			}
			current = active
		}
		if current.State != skillclaim.StateActive {
			return fmt.Errorf("recovery-required: cannot roll back claim %s in state %s", current.Destination, current.State)
		}
		operationID, err := newClaimOperationID()
		if err != nil {
			return err
		}
		previous, err := skillclaim.Digest(current)
		if err != nil {
			return err
		}
		releasing := current
		releasing.State = skillclaim.StateReleasing
		releasing.OperationID = operationID
		releasing.Generation++
		if err := store.CompareAndSwap(releasing, current.Generation, previous); err != nil {
			return err
		}
		previous, err = skillclaim.Digest(releasing)
		if err != nil {
			return err
		}
		if err := store.Remove(releasing.Destination, releasing.Producer, releasing.OperationID, releasing.Generation, previous); err != nil {
			return err
		}
	}
	return nil
}
