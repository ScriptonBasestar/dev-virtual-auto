package cli

import (
	"errors"
	"fmt"
)

// envBridgeError carries the stable machine code alongside the human message.
//
// TASK-245 §7-1 forbids a second root error envelope, so the code rides on the
// existing `{"error": …}` object as an optional key. A consumer that already
// reads `.error.message` and `.error.exit_code` is unaffected; one that wants to
// branch without parsing prose reads `.error.code`.
//
// The message always carries the `env bridge: ` prefix, which is why the
// constructor adds it rather than each call site: the prefix is what tells a
// reader of a bare stderr line which subsystem refused.
type envBridgeError struct {
	code    string
	message string
}

func (e *envBridgeError) Error() string { return "env bridge: " + e.message }

// Code returns the frozen machine code.
func (e *envBridgeError) Code() string { return e.code }

func bridgeErr(code, format string, args ...any) error {
	return &envBridgeError{code: code, message: fmt.Sprintf(format, args...)}
}

// errorCode extracts a stable code from an error chain, or "" when the failure
// has none. Everything outside the bridge keeps the old codeless envelope.
func errorCode(err error) string {
	if be, ok := errors.AsType[*envBridgeError](err); ok {
		return be.code
	}
	return ""
}

// The closed code set of TASK-245 §7-1. No code outside this list may be
// emitted; the table test in config_env_test.go asserts the set has not drifted,
// because an invented code would be indistinguishable from a frozen one to a
// consumer switching on the value.
const (
	codeNoEncryptedEntry    = "no_encrypted_env_entry"
	codeAmbiguousSelector   = "ambiguous_env_selector"
	codeUnknownTarget       = "unknown_env_target"
	codeTargetNotEncrypted  = "env_target_not_encrypted"
	codeJSONUnsupported     = "json_unsupported_for_edit"
	codeForceUnsupported    = "force_unsupported_for_edit"
	codeUnsupportedOrigin   = "unsupported_env_origin"
	codeUnknownOrigin       = "unknown_env_origin"
	codeAbsolutePath        = "absolute_path"
	codePathEscapes         = "path_escapes_config_root"
	codePathComponentSymlnk = "path_component_symlink"
	codeSourceMissing       = "source_missing"
	codeSourceNotRegular    = "source_not_regular"
	codeSourceUnreadable    = "source_unreadable"
	codeSourceIsTarget      = "source_is_target"
	codeTargetExists        = "target_exists"
	codeTargetNotRegular    = "target_not_regular"
	codeTargetTracked       = "target_tracked"
	codeTargetNotIgnored    = "target_not_ignored"
	codeTargetParentMissing = "target_parent_missing"
	codePermissionDenied    = "permission_denied"
	codeGitUnavailable      = "git_unavailable"
	codeSopsNotFound        = "sops_not_found"
	codeDecryptFailed       = "decrypt_failed"
	codeEmptyOutput         = "empty_decrypted_output"
	codeInvalidDotenv       = "invalid_dotenv_output"
	codeUnsupportedPlatform = "unsupported_platform"
)

// envBridgeCodes is the closed set as data, for the drift test.
var envBridgeCodes = []string{
	codeNoEncryptedEntry, codeAmbiguousSelector, codeUnknownTarget, codeTargetNotEncrypted,
	codeJSONUnsupported, codeForceUnsupported, codeUnsupportedOrigin, codeUnknownOrigin,
	codeAbsolutePath, codePathEscapes, codePathComponentSymlnk,
	codeSourceMissing, codeSourceNotRegular, codeSourceUnreadable, codeSourceIsTarget,
	codeTargetExists, codeTargetNotRegular, codeTargetTracked, codeTargetNotIgnored,
	codeTargetParentMissing, codePermissionDenied, codeGitUnavailable,
	codeSopsNotFound, codeDecryptFailed, codeEmptyOutput, codeInvalidDotenv,
	codeUnsupportedPlatform,
}
