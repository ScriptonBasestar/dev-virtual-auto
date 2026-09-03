package agentdeny

import (
	"encoding/json"
	"fmt"
)

// jsonObject preserves every field it doesn't understand as raw bytes, so a
// read-modify-write cycle never drops or reinterprets a value this package has no
// opinion about. Content survives exactly; only the touched containers' own
// presentation (indentation, key order) is normalized by re-marshaling — see
// docs/agent-deny-rules.md "What 'never clobbers' means here" for why that trade-off
// was accepted instead of byte-level source surgery.
type jsonObject = map[string]json.RawMessage

// claudeCodeDenyArray reads the current permissions.deny array from a Claude Code
// settings.json document without modifying anything. Returns (nil, nil) if the file is
// empty/absent (contents is empty) or declares no permissions.deny at all.
func claudeCodeDenyArray(contents []byte) ([]string, error) {
	top, err := loadJSONObject(contents)
	if err != nil {
		return nil, err
	}
	deny, _, err := readDenyArray(top)
	return deny, err
}

// mergeClaudeCodeDeny adds every pattern in add to the document's permissions.deny
// array that is not already present verbatim, preserving the existing array's order and
// every other key in the document (including sibling permissions.allow/permissions.ask
// entries) untouched at the value level. Returns the new document bytes, the resulting
// full deny array, and whether anything changed.
func mergeClaudeCodeDeny(contents []byte, add []string) (newContents []byte, finalDeny []string, changed bool, err error) {
	top, err := loadJSONObject(contents)
	if err != nil {
		return nil, nil, false, err
	}
	deny, perms, err := readDenyArray(top)
	if err != nil {
		return nil, nil, false, err
	}
	merged, changed := unionAppend(deny, add)
	if !changed {
		return contents, deny, false, nil
	}
	out, err := writeDenyArray(top, perms, merged)
	if err != nil {
		return nil, nil, false, err
	}
	return out, merged, true, nil
}

// removeClaudeCodeDeny removes every pattern in remove from the document's
// permissions.deny array that is present verbatim, leaving every other entry (including
// any the user added independently) and every other key untouched. Returns the new
// document bytes, the resulting full deny array, and how many patterns were actually
// removed (never more than len(remove); patterns already absent are simply skipped).
func removeClaudeCodeDeny(contents []byte, remove []string) (newContents []byte, finalDeny []string, removedCount int, err error) {
	top, err := loadJSONObject(contents)
	if err != nil {
		return nil, nil, 0, err
	}
	deny, perms, err := readDenyArray(top)
	if err != nil {
		return nil, nil, 0, err
	}
	remaining, removed := removeExact(deny, remove)
	if removed == 0 {
		return contents, deny, 0, nil
	}
	out, err := writeDenyArray(top, perms, remaining)
	if err != nil {
		return nil, nil, 0, err
	}
	return out, remaining, removed, nil
}

func loadJSONObject(contents []byte) (jsonObject, error) {
	if len(contents) == 0 {
		return jsonObject{}, nil
	}
	var top jsonObject
	if err := json.Unmarshal(contents, &top); err != nil {
		return nil, fmt.Errorf("parse settings JSON: %w", err)
	}
	if top == nil {
		top = jsonObject{}
	}
	return top, nil
}

func readDenyArray(top jsonObject) (deny []string, perms jsonObject, err error) {
	permsRaw, ok := top["permissions"]
	if !ok {
		return nil, nil, nil
	}
	if err := json.Unmarshal(permsRaw, &perms); err != nil {
		return nil, nil, fmt.Errorf("parse settings permissions: %w", err)
	}
	denyRaw, ok := perms["deny"]
	if !ok {
		return nil, perms, nil
	}
	if err := json.Unmarshal(denyRaw, &deny); err != nil {
		return nil, nil, fmt.Errorf("settings permissions.deny is not a string array: %w", err)
	}
	return deny, perms, nil
}

func writeDenyArray(top jsonObject, perms jsonObject, deny []string) ([]byte, error) {
	if perms == nil {
		perms = jsonObject{}
	}
	if top == nil {
		top = jsonObject{}
	}
	denyRaw, err := json.Marshal(deny)
	if err != nil {
		return nil, err
	}
	perms["deny"] = denyRaw
	permsRaw, err := json.Marshal(perms)
	if err != nil {
		return nil, err
	}
	top["permissions"] = permsRaw
	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func unionAppend(existing, add []string) (merged []string, changed bool) {
	have := make(map[string]bool, len(existing))
	for _, pattern := range existing {
		have[pattern] = true
	}
	merged = append([]string(nil), existing...)
	for _, pattern := range add {
		if !have[pattern] {
			merged = append(merged, pattern)
			have[pattern] = true
			changed = true
		}
	}
	return merged, changed
}

func removeExact(existing, remove []string) (remaining []string, removedCount int) {
	toRemove := make(map[string]bool, len(remove))
	for _, pattern := range remove {
		toRemove[pattern] = true
	}
	for _, pattern := range existing {
		if toRemove[pattern] {
			removedCount++
			continue
		}
		remaining = append(remaining, pattern)
	}
	return remaining, removedCount
}
