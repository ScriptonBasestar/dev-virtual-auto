package output

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// PrintJSON marshals data as indented JSON and prints to stdout.
func PrintJSON(data any) error {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(bytes))
	return nil
}

// PrintYAML marshals data as YAML and prints to stdout.
func PrintYAML(data any) error {
	bytes, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	fmt.Print(string(bytes))
	return nil
}
