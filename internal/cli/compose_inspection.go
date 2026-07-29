package cli

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func deduplicateComposeFiles(dir string, files []string) []string {
	unique := make([]string, 0, len(files))
	identities := make([]os.FileInfo, 0, len(files))

	for _, file := range files {
		path := file
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, file)
		}
		info, err := os.Stat(path)
		if err != nil {
			if !contains(unique, file) {
				unique = append(unique, file)
			}
			continue
		}

		duplicate := false
		for _, identity := range identities {
			if os.SameFile(info, identity) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}

		unique = append(unique, file)
		identities = append(identities, info)
	}

	return unique
}

func extractComposeServices(path string) []string {
	collector := composeServiceCollector{
		seen:       map[string]bool{},
		serviceSet: map[string]bool{},
	}
	collector.collect(path)
	return collector.services
}

type composeServiceCollector struct {
	seen       map[string]bool
	serviceSet map[string]bool
	services   []string
}

func (collector *composeServiceCollector) collect(path string) {
	canonicalPath := canonicalComposePath(path)
	if collector.seen[canonicalPath] {
		return
	}
	collector.seen[canonicalPath] = true

	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return
	}
	root := &document
	if document.Kind == yaml.DocumentNode && len(document.Content) == 1 {
		root = document.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return
	}

	for index := 0; index+1 < len(root.Content); index += 2 {
		key := root.Content[index]
		value := root.Content[index+1]
		switch key.Value {
		case "services":
			collector.appendServiceNames(value)
		case "include":
			for _, includePath := range composeIncludePaths(value) {
				collector.collect(filepath.Join(filepath.Dir(path), includePath))
			}
		}
	}
}

func canonicalComposePath(path string) string {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		absolutePath = filepath.Clean(path)
	}
	resolvedPath, err := filepath.EvalSymlinks(absolutePath)
	if err == nil {
		return resolvedPath
	}
	return filepath.Clean(absolutePath)
}

func (collector *composeServiceCollector) appendServiceNames(node *yaml.Node) {
	if node.Kind != yaml.MappingNode {
		return
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		name := node.Content[index].Value
		if name == "" || collector.serviceSet[name] {
			continue
		}
		collector.serviceSet[name] = true
		collector.services = append(collector.services, name)
	}
}

func composeIncludePaths(node *yaml.Node) []string {
	switch node.Kind {
	case yaml.AliasNode:
		if node.Alias == nil {
			return nil
		}
		return composeIncludePaths(node.Alias)
	case yaml.ScalarNode:
		if node.Tag == "!!str" && node.Value != "" {
			return []string{node.Value}
		}
	case yaml.SequenceNode:
		var paths []string
		for _, item := range node.Content {
			paths = append(paths, composeIncludePaths(item)...)
		}
		return paths
	case yaml.MappingNode:
		for index := 0; index+1 < len(node.Content); index += 2 {
			if node.Content[index].Value == "path" {
				return composeIncludePaths(node.Content[index+1])
			}
		}
	}
	return nil
}
