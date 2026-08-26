package main

import (
	"fmt"
	"sort"
	"strings"
)

var wrappedBindingTools = map[string]struct{}{
	"find": {},
	"grep": {},
}

// checkBindingTool rejects verify bindings that invoke a shell-wrapped tool by
// bare name. An absolute path makes the recorded result reproducible in an
// ordinary reader's shell instead of depending on an agent's function wrapper.
func checkBindingTool(from, body string) (checked, bare int, msgs []string) {
	for _, binding := range extractVerifyBindings(body) {
		wrapped, tools := bindingWrappedTools(binding.Span)
		if len(wrapped) == 0 {
			continue
		}
		checked++
		if len(tools) == 0 {
			continue
		}
		bare++
		msgs = append(msgs, fmt.Sprintf("%s:%d: verify binding invokes bare wrapped tool(s): %s", from, binding.Line, strings.Join(tools, ", ")))
	}
	return checked, bare, msgs
}

// bareWrappedTools lexes enough shell structure to recognize a command word
// without mistaking quoted patterns or later argv for commands. Operators and
// command substitutions open a new command position; assignments and shell
// control words do not consume it.
func bindingWrappedTools(span string) (wrapped, bare []string) {
	type quote byte
	const (
		unquoted quote = iota
		singleQuoted
		doubleQuoted
	)

	state := unquoted
	commandStart := true
	var commandRestore []bool
	var quoteRestore []quote
	var token strings.Builder
	redirectionOperand := false
	found := make(map[string]struct{})
	foundBare := make(map[string]struct{})

	consume := func() {
		if token.Len() == 0 {
			return
		}
		word := token.String()
		token.Reset()
		if !commandStart {
			return
		}
		if redirectionOperand {
			redirectionOperand = false
			return
		}
		if redirection, needsOperand := shellRedirection(word); redirection {
			redirectionOperand = needsOperand
			return
		}
		if isShellAssignment(word) || isCommandPrefix(word) {
			return
		}
		if _, ok := wrappedBindingTools[word]; ok {
			found[word] = struct{}{}
			foundBare[word] = struct{}{}
		} else if strings.HasPrefix(word, "/") {
			name := word[strings.LastIndexByte(word, '/')+1:]
			if _, ok := wrappedBindingTools[name]; ok {
				found[name] = struct{}{}
			}
		}
		commandStart = false
	}

	for i := 0; i < len(span); i++ {
		c := span[i]
		switch state {
		case singleQuoted:
			if c == '\'' {
				state = unquoted
			} else {
				token.WriteByte(c)
			}
			continue
		case doubleQuoted:
			if c == '"' {
				state = unquoted
				continue
			}
			if c == '\\' && i+1 < len(span) {
				next := span[i+1]
				if strings.ContainsRune("$`\"\\\n", rune(next)) {
					token.WriteByte(next)
					i++
					continue
				}
				token.WriteByte(c)
				continue
			}
			if c == '$' && i+1 < len(span) && span[i+1] == '(' {
				consume()
				commandRestore = append(commandRestore, commandStart)
				quoteRestore = append(quoteRestore, doubleQuoted)
				commandStart = true
				state = unquoted
				i++
				continue
			}
			token.WriteByte(c)
			continue
		}

		switch c {
		case '\'', '"':
			if c == '\'' {
				state = singleQuoted
			} else {
				state = doubleQuoted
			}
		case ' ', '\t', '\r':
			consume()
		case '\n', ';', '|', '&':
			consume()
			commandStart = true
		case '(':
			consume()
			commandRestore = append(commandRestore, commandStart)
			quoteRestore = append(quoteRestore, unquoted)
			commandStart = true
		case ')':
			consume()
			if len(commandRestore) > 0 {
				commandStart = commandRestore[len(commandRestore)-1]
				commandRestore = commandRestore[:len(commandRestore)-1]
				state = quoteRestore[len(quoteRestore)-1]
				quoteRestore = quoteRestore[:len(quoteRestore)-1]
			}
		case '!':
			if token.Len() == 0 && commandStart {
				continue
			}
			token.WriteByte(c)
		case '\\':
			if i+1 < len(span) {
				token.WriteByte(span[i+1])
				i++
			} else {
				token.WriteByte(c)
			}
		case '$':
			if i+1 < len(span) && span[i+1] == '(' {
				consume()
				commandRestore = append(commandRestore, commandStart)
				quoteRestore = append(quoteRestore, unquoted)
				commandStart = true
				i++
				continue
			}
			token.WriteByte(c)
		default:
			token.WriteByte(c)
		}
	}
	consume()

	tools := make([]string, 0, len(found))
	for tool := range found {
		tools = append(tools, tool)
	}
	sort.Strings(tools)
	bareTools := make([]string, 0, len(foundBare))
	for tool := range foundBare {
		bareTools = append(bareTools, tool)
	}
	sort.Strings(bareTools)
	return tools, bareTools
}

func isShellAssignment(word string) bool {
	i := strings.IndexByte(word, '=')
	if i <= 0 {
		return false
	}
	for j, r := range word[:i] {
		valid := r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || j > 0 && r >= '0' && r <= '9'
		if valid {
			continue
		}
		return false
	}
	return true
}

func isCommandPrefix(word string) bool {
	switch word {
	case "!", "{", "command", "do", "elif", "else", "if", "noglob", "then", "time", "until", "while":
		return true
	default:
		return false
	}
}

// shellRedirection reports redirection words accepted before a command name.
// A word containing only the operator consumes the following word as its target;
// combined forms such as </dev/null and 2>&1 carry their own target.
func shellRedirection(word string) (redirection, needsOperand bool) {
	i := 0
	for i < len(word) && word[i] >= '0' && word[i] <= '9' {
		i++
	}
	if i == len(word) || word[i] != '<' && word[i] != '>' {
		return false, false
	}
	j := i + 1
	for j < len(word) && (word[j] == '<' || word[j] == '>' || word[j] == '|' || word[j] == '&') {
		j++
	}
	return true, j == len(word)
}
