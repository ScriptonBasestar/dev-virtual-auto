package skillclaim

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestPortableClaimDigestVector(t *testing.T) {
	contents, err := os.ReadFile("testdata/valid-file-claim.json")
	if err != nil {
		t.Fatal(err)
	}
	claim, err := Decode(contents)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(claim, claim.Destination); err != nil {
		t.Fatal(err)
	}
	got, err := Digest(claim)
	if err != nil {
		t.Fatal(err)
	}
	const want = "e9d6ae74f9b56f32c974f3bcadcbc3548e8b77a197818ef0c1fe053ab916074c"
	if got != want {
		t.Fatalf("claim digest = %s, want %s", got, want)
	}
}

func validClaim(t *testing.T, destination, producer string) Claim {
	t.Helper()
	canonical, err := CanonicalDestination(destination)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("skill"))
	files := []FileHash{{Path: "SKILL.md", SHA: hex.EncodeToString(digest[:])}}
	manifest, err := ManifestDigest(files)
	if err != nil {
		t.Fatal(err)
	}
	return Claim{Schema: Schema, Name: "dva", Kind: KindDirectory, State: StateActive, OperationID: "test-operation", Generation: 1, Destination: canonical, Producer: producer, Format: "agent-skills-directory", Scope: "user", Consumers: []string{"codex"}, SourceDigest: manifest, Files: files}
}

func TestPathUsesNeutralXDGRoot(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "home", ".agents", "skills", "dva")
	want := filepath.Join(root, "agent-skills", "claims", "v1")
	if got := filepath.Dir(Path(root, destination)); got != want {
		t.Fatalf("claim path = %s, want %s", got, want)
	}
}

func TestWriteRefusesOtherProducer(t *testing.T) {
	root, destination := t.TempDir(), filepath.Join(t.TempDir(), "dva")
	if err := Reserve(root, validClaim(t, destination, "other")); err != nil {
		t.Fatal(err)
	}
	if err := Reserve(root, validClaim(t, destination, "dva")); err == nil {
		t.Fatal("Reserve replaced another producer's claim")
	}
}

func TestDecodeRejectsDuplicateUnknownAndTrailingJSON(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`{"schema":1,"schema":1}`), []byte(`{"unknown":true}`), []byte(`{} {}`),
	} {
		if _, err := Decode(data); err == nil {
			t.Fatalf("Decode accepted %s", data)
		}
	}
}

func TestTransitionUsesGenerationAndTombstoneFailClosed(t *testing.T) {
	root, destination := t.TempDir(), filepath.Join(t.TempDir(), "dva")
	claim := validClaim(t, destination, "producer")
	if err := Reserve(root, claim); err != nil {
		t.Fatal(err)
	}
	claim, _, err := Read(root, destination)
	if err != nil {
		t.Fatal(err)
	}
	previous, err := Digest(claim)
	if err != nil {
		t.Fatal(err)
	}
	claim.Generation, claim.State = 2, StateActive
	if err := Activate(root, claim, 1, previous); err != nil {
		t.Fatal(err)
	}
	claim.State, claim.Generation = StateUpdating, 3
	claim.OperationID = "update-operation"
	previous, _ = Digest(Claim{Schema: Schema, Name: "dva", Kind: KindDirectory, State: StateActive, OperationID: "test-operation", Generation: 2, Destination: claim.Destination, Producer: "producer", Format: "agent-skills-directory", Scope: "user", Consumers: []string{"codex"}, SourceDigest: claim.SourceDigest, Files: claim.Files})
	if err := Transition(root, claim, 2, previous); err != nil {
		t.Fatal(err)
	}
	claim.State, claim.Generation = StateActive, 4
	claim.OperationID = "other-operation"
	previous, _ = Digest(Claim{Schema: Schema, Name: "dva", Kind: KindDirectory, State: StateUpdating, OperationID: "update-operation", Generation: 3, Destination: claim.Destination, Producer: "producer", Format: "agent-skills-directory", Scope: "user", Consumers: []string{"codex"}, SourceDigest: claim.SourceDigest, Files: claim.Files})
	if err := Transition(root, claim, 3, previous); err == nil {
		t.Fatal("Transition advanced non-active foreign operation")
	}
}

func TestFileClaimRequiresDotRecord(t *testing.T) {
	claim := validClaim(t, filepath.Join(t.TempDir(), "dva.md"), "producer")
	claim.Kind, claim.Files[0].Path = KindFile, "dva.md"
	if err := Validate(claim, claim.Destination); err == nil {
		t.Fatal("file claim accepted non-dot record")
	}
	claim.Files[0].Path = "."
	claim.SourceDigest, _ = ManifestDigest(claim.Files)
	if err := Validate(claim, claim.Destination); err != nil {
		t.Fatal(err)
	}
}

func TestCanonicalDestinationRejectsFinalSymlink(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(root, "dva")
	if err := os.Symlink(filepath.Join(root, "target"), link); err != nil {
		t.Fatal(err)
	}
	if _, err := CanonicalDestination(link); err == nil {
		t.Fatal("final destination symlink accepted")
	}
}

func TestLockedStoreReservesTwoClaimsAndRefusesDoubleClose(t *testing.T) {
	root, parent := t.TempDir(), t.TempDir()
	first, second := filepath.Join(parent, "dva"), filepath.Join(parent, "dva-config")
	store, err := Begin(root, []string{second, first, first})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Reserve(validClaim(t, first, "producer")); err != nil {
		t.Fatal(err)
	}
	secondClaim := validClaim(t, second, "producer")
	secondClaim.Name = "dva-config"
	if err := store.Reserve(secondClaim); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err == nil {
		t.Fatal("double Close succeeded")
	}
}

func TestLockedStoreRejectsUnauthorizedAndBadGeneration(t *testing.T) {
	root, parent := t.TempDir(), t.TempDir()
	allowed, outside := filepath.Join(parent, "dva"), filepath.Join(parent, "other")
	store, err := Begin(root, []string{allowed})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, _, err := store.Read(outside); err == nil {
		t.Fatal("read outside locked destinations")
	}
	claim := validClaim(t, allowed, "producer")
	claim.Generation = 9
	if err := store.Reserve(claim); err != nil {
		t.Fatal(err)
	}
	stored, _, err := store.Read(allowed)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Generation != 1 || stored.State != StateReserved {
		t.Fatalf("reserved = %#v", stored)
	}
	previous, _ := Digest(stored)
	stored.State, stored.Generation = StateActive, 3
	if err := store.CompareAndSwap(stored, 1, previous); err == nil {
		t.Fatal("CAS accepted generation jump")
	}
}
