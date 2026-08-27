package skillinstall

import (
	"fmt"
	"sort"
)

// TakeoverBackupResult is one verified takeover backup group. A backup ID is
// unique only within its destination, so callers must retain both fields.
type TakeoverBackupResult struct {
	BackupID    string    `json:"backup_id"`
	Destination string    `json:"destination"`
	Runtimes    []Runtime `json:"runtimes"`
	Skills      []string  `json:"skills"`
	Status      string    `json:"status"`
}

// BackupListResult contains the verified takeover backups selected by scope and
// runtime. Listing is read-only: it never creates, repairs, or removes state.
type BackupListResult struct {
	Scope   Scope                  `json:"scope"`
	Backups []TakeoverBackupResult `json:"backups"`
}

// ListTakeoverBackups reports every receipt-backed takeover backup for the
// selected destinations. resolve groups runtimes that share one destination,
// so Codex and Antigravity cannot produce duplicate rows for .agents/skills.
func ListTakeoverBackups(options Options) (BackupListResult, error) {
	resolved, destinations, err := resolve(options)
	if err != nil {
		return BackupListResult{}, err
	}
	result := BackupListResult{Scope: resolved.Scope, Backups: make([]TakeoverBackupResult, 0)}
	for _, target := range destinations {
		record, found, err := readReceipt(receiptPath(resolved.StateRoot, target.path))
		if err != nil {
			return BackupListResult{}, fmt.Errorf("read receipt for %s: %w", target.path, err)
		}
		if !found || len(record.Takeovers) == 0 {
			continue
		}
		if err := validateReceipt(record, resolved.Scope, target); err != nil {
			return BackupListResult{}, fmt.Errorf("validate receipt for %s: %w", target.path, err)
		}

		status, _ := verifyTakeoverBackups(resolved.StateRoot, record)
		byID := make(map[string][]string)
		for _, takeover := range record.Takeovers {
			byID[takeover.BackupID] = append(byID[takeover.BackupID], takeover.Skill)
		}
		ids := make([]string, 0, len(byID))
		for backupID := range byID {
			ids = append(ids, backupID)
		}
		sort.Strings(ids)
		for _, backupID := range ids {
			skills := byID[backupID]
			sort.Strings(skills)
			result.Backups = append(result.Backups, TakeoverBackupResult{
				BackupID: backupID, Destination: target.path,
				Runtimes: append([]Runtime(nil), target.runtimes...),
				Skills:   skills, Status: status,
			})
		}
	}
	return result, nil
}
