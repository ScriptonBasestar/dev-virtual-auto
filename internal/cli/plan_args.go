package cli

func planRoutingArgs(args []string) []string {
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--debug" || arg == "--json" {
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered
}
