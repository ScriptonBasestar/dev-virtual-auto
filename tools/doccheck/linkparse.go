package main

import (
	"net/url"
	"strings"
)

// splitLink separates path and anchor. Empty path means same-document (#foo).
func splitLink(target string) (path, anchor string, err error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", "", nil
	}
	if strings.HasPrefix(target, "#") {
		return "", decodeFragment(target[1:]), nil
	}
	filePart, frag, hasFrag := strings.Cut(target, "#")
	p, e := url.PathUnescape(filePart)
	if e != nil {
		return "", "", e
	}
	if !hasFrag {
		return p, "", nil
	}
	return p, decodeFragment(frag), nil
}

func decodeFragment(frag string) string {
	if u, err := url.PathUnescape(frag); err == nil {
		return u
	}
	return frag
}

func isExternalLink(target string) bool {
	t := strings.TrimSpace(target)
	if t == "" {
		return false
	}
	if i := strings.IndexByte(t, ':'); i > 0 {
		scheme := t[:i]
		if !strings.Contains(scheme, "/") && !strings.Contains(scheme, "#") {
			return true
		}
	}
	return strings.HasPrefix(t, "//")
}
