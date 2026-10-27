package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type builtinCompleter struct {
	names      []string
	lastPrefix string
}

func (c *builtinCompleter) Do(line []rune, pos int) ([][]rune, int) {
	text := string(line[:pos])

	lastSpaceIdx := strings.LastIndex(text, " ")
	isCommand := lastSpaceIdx == -1

	var prefix string
	if isCommand {
		prefix = text
	} else {
		prefix = text[lastSpaceIdx+1:]
	}

	matches := NewSet[string]()

	if isCommand {
		for _, name := range c.names {
			if strings.HasPrefix(name, prefix) {
				matches.Add(name)
			}
		}

		for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
			if dir == "" {
				dir = "."
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}

			for _, entry := range entries {
				name := entry.Name()
				if !strings.HasPrefix(name, prefix) {
					continue
				}

				info, err := os.Stat(filepath.Join(dir, name))
				if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
					continue
				}

				matches.Add(name)
			}
		}
	} else {
		dir, base := filepath.Split(prefix)
		searchDir := dir
		if searchDir == "" {
			searchDir = "."
		}
		searchDir = replaceTilde(searchDir)

		entries, err := os.ReadDir(searchDir)
		if err == nil {
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), base) {
					name := entry.Name()
					if entry.IsDir() {
						name += string(filepath.Separator)
					}
					fullMatch := dir + name
					matches.Add(fullMatch)
				}
			}
		}
	}

	if len(matches) == 0 {
		fmt.Print("\x07") // bell
		return nil, 0
	}

	names := make([]string, 0, len(matches))
	for name := range matches {
		names = append(names, name)
	}
	slices.Sort(names)

	lcp := longestCommonPrefix(names)

	if len(names) == 1 {
		c.lastPrefix = ""
		appendChar := " "
		if strings.HasSuffix(names[0], string(filepath.Separator)) {
			appendChar = ""
		}
		return [][]rune{[]rune(names[0][len(prefix):] + appendChar)}, len(prefix)
	}

	if len(lcp) > len(prefix) {
		c.lastPrefix = ""
		return [][]rune{[]rune(lcp[len(prefix):])}, len(prefix)
	}

	if prefix != c.lastPrefix {
		c.lastPrefix = prefix
		fmt.Print("\x07") // bell
		return nil, 0
	}

	c.lastPrefix = ""
	fmt.Print("\r\n" + strings.Join(names, "  ") + "\r\n")
	fmt.Print("$ " + text)
	return nil, 0

}

func longestCommonPrefix(names []string) string {
	if len(names) == 0 {
		return ""
	}

	first := names[0]
	last := names[len(names)-1]

	i := 0
	for i < len(first) && i < len(last) && first[i] == last[i] {
		i++
	}

	return first[:i]
}
