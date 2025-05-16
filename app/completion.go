package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type shellCompleter struct{
	lastTabPrefix string
    isFirstTab   bool
	lastCandidates []string
}

func (c *shellCompleter) Do(line []rune, pos int) (newLine [][]rune, length int) {
    lineStr := string(line[:pos])
    words := strings.Fields(lineStr)

    prefix := ""
    if len(words) > 0 && !strings.HasSuffix(lineStr, " ") {
        prefix = words[len(words)-1] 
    }

    if len(words) == 0 || (len(words) == 1 && !strings.HasSuffix(lineStr, " ")) {
		if prefix != c.lastTabPrefix {
			c.lastTabPrefix = prefix
			c.isFirstTab = true
			c.lastCandidates = getMatchingCommands(prefix)
		}
        candidates := c.lastCandidates
		sort.Strings(candidates)

        if len(candidates) > 1 {
			if c.isFirstTab {
				c.isFirstTab = false
				return [][]rune{[]rune("\a")}, 0
			} else {
				c.isFirstTab = true
				fmt.Print("\n")
				fmt.Print(strings.Join(candidates, "  "))
				fmt.Print("\n")
				return [][]rune{[]rune(prefix)}, len(prefix)
			}
		}

        if len(candidates) == 1 {
            return [][]rune{[]rune(candidates[0][len(prefix):] + " ")}, len(prefix)
        }
    }

    wordCount := len(words)

    if wordCount == 0 || (wordCount == 1 && !strings.HasSuffix(lineStr, " ")) {
        return completeCommand(words)
    } else {
        return completeArguement(lineStr, words)
    }
}


func completeCommand(words []string) ([][]rune, int) {
	prefix := ""
	if len(words) == 1{
		prefix = words[0]
	}

	candidates := []string{}

	for _, cmd := range builtinCommands {
		if strings.HasPrefix(cmd, prefix) {
			candidates = append(candidates, cmd)
		}
	}

	pathEnv := os.Getenv("PATH")
	paths := strings.Split(pathEnv, string(os.PathListSeparator))

	seen := make(map[string] bool)
	for _, candidate := range candidates {
		seen[candidate] = true
	}

	for _, dir := range paths {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, prefix) && !seen[name] {
				info, err := entry.Info()
				if err == nil && isExecutable(info.Mode()) {
					candidates = append(candidates, name)
					seen[name] = true
				}
			}
		}
	}

	if len(candidates) == 0 && prefix != "" {
		return [][]rune{[]rune("\a")}, 0
	}
	
	if len(candidates) == 0 {
		return nil, 0
	}

	return formatCompletionResults(prefix, candidates)
}

func completeArguement(lineStr string, words []string) ([][]rune, int) {

	partial := ""
	if strings.HasSuffix(lineStr, " ") {
		partial = ""
	} else {
		partial = words[len(words) - 1]
	}

	searchDir := "." 
	partialBase := filepath.Base(partial)

	if filepath.IsAbs(partial) {
		searchDir = filepath.Dir(partial)
	} else if partial != partialBase {
		searchDir = filepath.Dir(partial)
	}

	if strings.HasPrefix(partial, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			partial = filepath.Join(home, partial[2:])
			searchDir = filepath.Dir(partial)
			partialBase = filepath.Base(partial)
		}
	}

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return nil, 0
	}

	candidates := []string{}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, partialBase) {
			fullPath := filepath.Join(searchDir, name)

			if entry.IsDir() {
				fullPath += string(os.PathSeparator)
			}

			if filepath.IsAbs(partial) || partial != partialBase {
				candidates = append(candidates, fullPath[len(searchDir) - 1:])
			} else {
				candidates = append(candidates, name)
			}
		}
	}

	if len(candidates) == 0 {
		return nil, 0
	}	

	prefix := partial
	return formatCompletionResults(prefix, candidates)

}

func formatCompletionResults(prefix string, candidates []string) ([][]rune, int) {
	
	if len(candidates) == 1 {
		
		completion := candidates[0][len(prefix):]
		if !strings.HasSuffix(candidates[0], string(os.PathSeparator)) {
			completion += " "
		}
		return [][]rune{[]rune(completion)}, len(prefix)
	} else {	
		completions := make([][]rune, len(candidates))
		for i, candidate := range candidates {
			completions[i] = []rune(candidate[len(prefix):])
		}
		return completions, len(prefix)
	}
}

func getMatchingCommands(prefix string) []string {
	var candidates []string

	for _, cmd := range builtinCommands {
		if strings.HasPrefix(cmd, prefix) {
			candidates = append(candidates, cmd)
		}
	}

	pathDirs := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
	seen := make(map[string]bool)

	for _, dir := range pathDirs {
		
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, file := range files {
			name := file.Name()
			if strings.HasPrefix(name, prefix) && !seen[name] {
				info, err := file.Info()
				if err == nil && isExecutable(info.Mode()) {
					candidates = append(candidates, name)
					seen[name] = true
				}
			}
		}
	}
	return candidates
}

func isExecutable(mode fs.FileMode) bool {
	return mode&0111 != 0
}

