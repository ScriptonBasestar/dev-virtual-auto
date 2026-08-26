// Package skillclaim defines a strict, producer-neutral Agent Skills claim protocol.
package skillclaim

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

const Schema = 1
const (
	KindDirectory  = "directory"
	KindFile       = "file"
	StateReserved  = "reserved"
	StateActive    = "active"
	StateUpdating  = "updating"
	StateReleasing = "releasing"
	StateRestoring = "restoring"
)

type FileHash struct {
	Path string `json:"path"`
	SHA  string `json:"sha256"`
}
type Claim struct {
	Schema       int        `json:"schema"`
	Name         string     `json:"name"`
	Kind         string     `json:"kind"`
	State        string     `json:"state"`
	OperationID  string     `json:"operation_id"`
	Generation   uint64     `json:"generation"`
	Destination  string     `json:"destination"`
	Producer     string     `json:"producer"`
	Format       string     `json:"format"`
	Scope        string     `json:"scope"`
	Consumers    []string   `json:"consumers"`
	SourceDigest string     `json:"source_digest"`
	Files        []FileHash `json:"files"`
}

// Path takes the neutral XDG state root, never a producer state directory.
func Path(root, destination string) string {
	sum := sha256.Sum256([]byte(destination))
	return filepath.Join(root, "agent-skills", "claims", "v1", hex.EncodeToString(sum[:])+".json")
}

// CanonicalDestination resolves the nearest existing ancestor, including its symlinks, then appends a missing tail.
func CanonicalDestination(destination string) (string, error) {
	current, err := filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	var tail []string
	for {
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				if len(tail) == 0 {
					return "", fmt.Errorf("skill destination %s is a symlink", current)
				}
				current, err = filepath.EvalSymlinks(current)
				if err != nil {
					return "", err
				}
			}
			if resolved, err := filepath.EvalSymlinks(current); err == nil {
				current = resolved
			}
			for i := len(tail) - 1; i >= 0; i-- {
				current = filepath.Join(current, tail[i])
			}
			return filepath.Clean(current), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		tail = append(tail, filepath.Base(current))
		current = parent
	}
}

func Read(root, destination string) (Claim, bool, error) {
	destination, err := CanonicalDestination(destination)
	if err != nil {
		return Claim{}, false, err
	}
	path := Path(root, destination)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return Claim{}, false, nil
	}
	if err != nil {
		return Claim{}, false, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return Claim{}, false, fmt.Errorf("claim %s is not a regular file", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Claim{}, false, err
	}
	claim, err := Decode(data)
	if err != nil {
		return Claim{}, false, err
	}
	return claim, true, Validate(claim, destination)
}

func Decode(data []byte) (Claim, error) {
	if err := noDuplicateKeys(data); err != nil {
		return Claim{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var claim Claim
	if err := decoder.Decode(&claim); err != nil {
		return Claim{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Claim{}, errors.New("claim has trailing JSON value")
		}
		return Claim{}, err
	}
	return claim, nil
}
func noDuplicateKeys(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	var parse func() error
	parse = func() error {
		token, err := d.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok {
					return errors.New("claim object key is not a string")
				}
				if seen[name] {
					return fmt.Errorf("claim contains duplicate key %q", name)
				}
				seen[name] = true
				if err := parse(); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		case '[':
			for d.More() {
				if err := parse(); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		}
		return nil
	}
	if err := parse(); err != nil {
		return err
	}
	return nil
}

func Validate(c Claim, destination string) error {
	destination, err := CanonicalDestination(destination)
	if err != nil {
		return err
	}
	if c.Schema != Schema || c.Destination != destination {
		return errors.New("claim does not bind canonical destination")
	}
	if !token(c.Name) || !token(c.Producer) || !token(c.Format) || !token(c.Scope) || !token(c.OperationID) {
		return errors.New("claim has invalid identity metadata")
	}
	if c.Kind != KindDirectory && c.Kind != KindFile {
		return errors.New("claim has invalid kind")
	}
	if c.State != StateReserved && c.State != StateActive && c.State != StateUpdating && c.State != StateReleasing && c.State != StateRestoring {
		return errors.New("claim has invalid state")
	}
	if c.Generation == 0 || !digest(c.SourceDigest) || len(c.Consumers) == 0 || len(c.Files) == 0 {
		return errors.New("claim has invalid source metadata")
	}
	if !sortedUnique(c.Consumers) || !sort.SliceIsSorted(c.Files, func(i, j int) bool { return c.Files[i].Path < c.Files[j].Path }) {
		return errors.New("claim lists are not sorted and unique")
	}
	for _, consumer := range c.Consumers {
		if !token(consumer) {
			return errors.New("claim has invalid consumer")
		}
	}
	for _, file := range c.Files {
		if !pathRecord(file.Path, c.Kind) || !digest(file.SHA) {
			return errors.New("claim has invalid file record")
		}
	}
	return nil
}
func token(value string) bool {
	return value != "" && !strings.ContainsAny(value, "/\\") && !strings.ContainsFunc(value, unicode.IsControl)
}
func pathRecord(value, kind string) bool {
	if kind == KindFile {
		return value == "."
	}
	return value != "" && !filepath.IsAbs(value) && filepath.Clean(value) == value && value != ".." && !strings.HasPrefix(value, ".."+string(filepath.Separator)) && !strings.ContainsFunc(value, unicode.IsControl)
}
func digest(value string) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == sha256.Size && value == strings.ToLower(value)
}
func sortedUnique(values []string) bool {
	if !sort.StringsAreSorted(values) {
		return false
	}
	for i := 1; i < len(values); i++ {
		if values[i-1] == values[i] {
			return false
		}
	}
	return true
}

type LockSet struct {
	paths    []string
	released bool
}

func AcquireLocks(root string, destinations []string) (*LockSet, error) {
	unique := map[string]bool{}
	paths := make([]string, 0, len(destinations))
	for _, destination := range destinations {
		canonical, err := CanonicalDestination(destination)
		if err != nil {
			return nil, err
		}
		path := Path(root, canonical) + ".lock"
		if !unique[path] {
			unique[path] = true
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	if len(paths) > 0 {
		if err := os.MkdirAll(filepath.Dir(paths[0]), 0o700); err != nil {
			return nil, err
		}
	}
	for index, path := range paths {
		if err := os.Mkdir(path, 0o700); err != nil {
			for i := index - 1; i >= 0; i-- {
				_ = os.Remove(paths[i])
			}
			if errors.Is(err, os.ErrExist) {
				return nil, fmt.Errorf("claim mutation lock exists at %s", path)
			}
			return nil, err
		}
	}
	return &LockSet{paths: paths}, nil
}
func (locks *LockSet) Release() error {
	if locks == nil || locks.released {
		return errors.New("claim locks already released")
	}
	locks.released = true
	var first error
	for i := len(locks.paths) - 1; i >= 0; i-- {
		if err := os.Remove(locks.paths[i]); err != nil && !errors.Is(err, os.ErrNotExist) && first == nil {
			first = err
		}
	}
	return first
}

func Digest(claim Claim) (string, error) {
	data, err := json.Marshal(claim)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// Transition is the single-claim convenience around a LockedStore transaction.
func Transition(root string, next Claim, expectedGeneration uint64, previousDigest string) error {
	canonical, err := CanonicalDestination(next.Destination)
	if err != nil {
		return err
	}
	next.Destination = canonical
	if err := Validate(next, canonical); err != nil {
		return err
	}
	locks, err := AcquireLocks(root, []string{canonical})
	if err != nil {
		return err
	}
	defer locks.Release()
	store := &LockedStore{root: root, locks: locks}
	return store.CompareAndSwap(next, expectedGeneration, previousDigest)
}

// Reserve persists a new reserved claim with O_EXCL. Call Activate with the same operation ID.
func Reserve(root string, claim Claim) error {
	claim.State = StateReserved
	if claim.Generation == 0 {
		claim.Generation = 1
	}
	return Transition(root, claim, 0, "")
}
func Activate(root string, claim Claim, expectedGeneration uint64, previousDigest string) error {
	claim.State = StateActive
	return Transition(root, claim, expectedGeneration, previousDigest)
}

// LockedStore is a multi-claim transaction boundary. Its locks must be acquired once, in canonical order.
type LockedStore struct {
	root  string
	locks *LockSet
}

func Begin(root string, destinations []string) (*LockedStore, error) {
	locks, err := AcquireLocks(root, destinations)
	if err != nil {
		return nil, err
	}
	return &LockedStore{root: root, locks: locks}, nil
}
func (store *LockedStore) Close() error { return store.locks.Release() }
func (store *LockedStore) Read(destination string) (Claim, bool, error) {
	return Read(store.root, destination)
}
func (store *LockedStore) Reserve(claim Claim) error {
	claim.State = StateReserved
	if claim.Generation == 0 {
		claim.Generation = 1
	}
	return store.CompareAndSwap(claim, 0, "")
}
func (store *LockedStore) CompareAndSwap(next Claim, generation uint64, previous string) error {
	canonical, err := CanonicalDestination(next.Destination)
	if err != nil {
		return err
	}
	next.Destination = canonical
	if err := Validate(next, canonical); err != nil {
		return err
	}
	current, found, err := Read(store.root, canonical)
	if err != nil {
		return err
	}
	if !found {
		if generation != 0 || previous != "" || next.State != StateReserved {
			return errors.New("new claim must be reserved with empty predecessor")
		}
		return write(store.root, next, false)
	}
	digest, err := Digest(current)
	if err != nil {
		return err
	}
	if current.Generation != generation || digest != previous || current.Producer != next.Producer {
		return errors.New("claim reservation changed")
	}
	if !allowedTransition(current, next) {
		return errors.New("claim state or operation transition is invalid")
	}
	return write(store.root, next, true)
}
func (store *LockedStore) Remove(destination string, producer string, generation uint64, previous string) error {
	current, found, err := Read(store.root, destination)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("claim reservation is absent")
	}
	digest, err := Digest(current)
	if err != nil {
		return err
	}
	if current.Producer != producer || current.Generation != generation || digest != previous || current.State != StateReleasing {
		return errors.New("claim removal precondition failed")
	}
	if err := os.Remove(Path(store.root, current.Destination)); err != nil {
		return err
	}
	return syncDir(filepath.Dir(Path(store.root, current.Destination)))
}
func allowedTransition(from, to Claim) bool {
	if from.State == StateReserved {
		return to.State == StateActive && to.OperationID == from.OperationID
	}
	if from.State == StateActive {
		return (to.State == StateUpdating || to.State == StateReleasing || to.State == StateRestoring)
	}
	if from.OperationID != to.OperationID {
		return false
	}
	return (from.State == StateUpdating || from.State == StateRestoring) && to.State == StateActive
}
func Write(root string, next Claim) error {
	current, found, err := Read(root, next.Destination)
	if err != nil {
		return err
	}
	if !found {
		return Transition(root, next, 0, "")
	}
	if current.Producer != next.Producer {
		return fmt.Errorf("claim for %s belongs to producer %q", next.Destination, current.Producer)
	}
	previous, err := Digest(current)
	if err != nil {
		return err
	}
	return Transition(root, next, current.Generation, previous)
}
func Remove(root, destination, producer string) error {
	current, found, err := Read(root, destination)
	if err != nil || !found {
		return err
	}
	if current.Producer != producer {
		return fmt.Errorf("claim for %s belongs to producer %q", destination, current.Producer)
	}
	locks, err := AcquireLocks(root, []string{current.Destination})
	if err != nil {
		return err
	}
	defer locks.Release()
	return os.Remove(Path(root, current.Destination))
}
func write(root string, claim Claim, replace bool) error {
	path := Path(root, claim.Destination)
	data, err := json.MarshalIndent(claim, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if !replace {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		_, writeErr := file.Write(data)
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
		return syncPath(path)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".claim-")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err = temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	return syncPath(path)
}
func syncPath(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	err = file.Sync()
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return syncDir(filepath.Dir(path))
}
func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	err = dir.Sync()
	closeErr := dir.Close()
	if err != nil {
		return err
	}
	return closeErr
}
