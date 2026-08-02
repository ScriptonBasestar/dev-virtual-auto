package config

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

// walkMark records where a node sits relative to the walk currently in progress.
type walkMark uint8

const (
	markOnPath  walkMark = iota + 1 // an ancestor of the node being visited
	markCleared                     // fully walked from an earlier position, no cycle below it
)

// checkAnchorCycles rejects a document in which an anchor is reachable from itself,
// naming the anchor and the path at which it closes the loop.
//
// This has to run before the document is decoded into config types.
// InteractionCommand.UnmarshalYAML decodes its subcommands through (*Node).Decode,
// which allocates a fresh decoder — and therefore a fresh copy of yaml.v3's own alias
// guard — at every re-entry, so that guard never fires and a self-referencing anchor
// recurses without bound. The result is a runtime stack overflow, which is a fatal
// error rather than a panic: no recover() up the stack can contain it, so the only
// possible defense is one that runs first. Parsing to a Node never enters user types,
// so this walk is safe on the bytes the decoder would die on. TASK-131.
//
// Reachability alone is not a cycle: only a node that is its own ancestor is. Shared
// anchors and merge keys (`<<: *base`) legitimately reach one node from several
// places, so the walk tracks the current path rather than everything it has seen.
func checkAnchorCycles(root *yaml.Node) error {
	marks := map[*yaml.Node]walkMark{}

	var walk func(n *yaml.Node, path string) error
	walk = func(n *yaml.Node, path string) error {
		if n == nil {
			return nil
		}
		switch marks[n] {
		case markOnPath:
			return fmt.Errorf("anchor '%s' contains itself at %s.\n  Hint: a YAML anchor cannot alias one of its own ancestors — remove the self-reference",
				anchorLabel(n), pathLabel(path))
		case markCleared:
			// Clearing lets a document that aliases one anchor from many places stay
			// linear in the number of nodes instead of exponential in the nesting.
			return nil
		}

		marks[n] = markOnPath
		defer func() { marks[n] = markCleared }()

		// An alias stands in for its referent, so it reports the same path.
		if n.Alias != nil {
			return walk(n.Alias, path)
		}

		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				key, value := n.Content[i], n.Content[i+1]
				// A key is addressed by the mapping that holds it, not by its own value.
				if err := walk(key, path); err != nil {
					return err
				}
				if err := walk(value, childPath(path, key.Value)); err != nil {
					return err
				}
			}
			return nil
		}

		for i, child := range n.Content {
			// Sequence entries and the single child of a document node; a document has
			// no name of its own, so its child keeps the root path.
			next := path
			if n.Kind == yaml.SequenceNode {
				next = childPath(path, "["+strconv.Itoa(i)+"]")
			}
			if err := walk(child, next); err != nil {
				return err
			}
		}
		return nil
	}

	return walk(root, "")
}

// anchorLabel names the node a cycle closes on. Only an anchored node can be
// reached twice, so the fallbacks cover malformed input rather than valid YAML.
func anchorLabel(n *yaml.Node) string {
	switch {
	case n.Anchor != "":
		return n.Anchor
	case n.Value != "":
		return n.Value
	default:
		return "<unnamed>"
	}
}

func pathLabel(path string) string {
	if path == "" {
		return "the document root"
	}
	return path
}

func childPath(path, name string) string {
	switch {
	case path == "":
		return name
	case name != "" && name[0] == '[':
		return path + name
	default:
		return path + "." + name
	}
}
