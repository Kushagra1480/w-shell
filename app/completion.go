package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sort"
)

var (
	lastTabPrefix string
	lastCandidates []string
	isTabPressed bool
	completionTrie *Trie
	completionState *CompletionState
)


type TrieNode struct {
	children map[rune]*TrieNode
	isEnd    bool
	word     string
}


type Trie struct {
	root *TrieNode
}


type CompletionState struct {
	currentPrefix string
	currentNode   *TrieNode
	candidates    []string
}


func NewTrie() *Trie {
	return &Trie{
		root: &TrieNode{
			children: make(map[rune]*TrieNode),
			isEnd:    false,
		},
	}
}


func (t *Trie) Insert(word string) {
	node := t.root
	
	for _, ch := range word {
		if _, exists := node.children[ch]; !exists {
			node.children[ch] = &TrieNode{
				children: make(map[rune]*TrieNode),
				isEnd:    false,
			}
		}
		node = node.children[ch]
	}
	node.isEnd = true
	node.word = word
}


func (t *Trie) FindWithPrefix(prefix string) []string {
	var results []string
	node := t.findPrefixNode(prefix)
	
	if node == nil {
		return results
	}
	
	t.collectAllWords(node, &results)
	return results
}


func (t *Trie) findPrefixNode(prefix string) *TrieNode {
	node := t.root
	
	for _, ch := range prefix {
		if child, exists := node.children[ch]; exists {
			node = child
		} else {
			return nil
		}
	}
	
	return node
}


func (t *Trie) collectAllWords(node *TrieNode, results *[]string) {
	if node.isEnd {
		*results = append(*results, node.word)
	}
	
	for _, child := range node.children {
		t.collectAllWords(child, results)
	}
}


func (t *Trie) FindLongestCommonPrefix(prefix string) string {
	node := t.findPrefixNode(prefix)
	if node == nil {
		return prefix
	}
	
	
	for len(node.children) == 1 && !node.isEnd {
		for r, child := range node.children {
			prefix += string(r)
			node = child
		}
	}
	
	return prefix
}


func (t *Trie) FindImmediateCompletions(prefix string) []string {
	node := t.findPrefixNode(prefix)
	if node == nil {
		return nil
	}
	
	var completions []string
	
	
	if node.isEnd {
		completions = append(completions, node.word)
	}
	
	
	for _, child := range node.children {
		if child.isEnd {
			completions = append(completions, child.word)
		}
	}
	
	return completions
}


func (t *Trie) FindNextCompletion(prefix string, currentState *CompletionState) string {
	
	if currentState != nil && currentState.currentPrefix == prefix {
		node := currentState.currentNode
		
		
		if node == nil {
			return ""
		}
		
		
		var candidates []string
		t.collectAllWords(node, &candidates)
		
		
		
		if len(candidates) > 1 {
			
			longestPrefix := findLongestCommonPrefix(candidates)
			
			
			if longestPrefix == prefix {
				
				return ""
			}
			
			
			currentState.currentPrefix = longestPrefix
			currentState.currentNode = t.findPrefixNode(longestPrefix)
			return longestPrefix
		}
	}
	
	
	node := t.findPrefixNode(prefix)
	if node == nil {
		return ""
	}
	
	
	var candidates []string
	t.collectAllWords(node, &candidates)
	
	
	if len(candidates) == 0 {
		return ""
	}
	
	
	if len(candidates) == 1 {
		return candidates[0]
	}
	
	
	longestPrefix := findLongestCommonPrefix(candidates)
	
	
	if longestPrefix == prefix {
		return ""
	}
	
	
	if currentState != nil {
		currentState.currentNode = t.findPrefixNode(longestPrefix)
		currentState.currentPrefix = longestPrefix
	}
	
	return longestPrefix
}

type shellCompleter struct{}

func (c *shellCompleter) Do(line []rune, pos int) (newLine [][]rune, length int) {
	lineStr := string(line[:pos])
	words := strings.Fields(lineStr)
	
	prefix := ""
	if len(words) > 0 && !strings.HasSuffix(lineStr, " ") {
		prefix = words[len(words)-1]
	}
	
	prefix = strings.Map(func(r rune) rune {
		if r < 32 {
			return -1
		}
		return r
	}, prefix)
	
	
	if completionTrie == nil {
		completionTrie = buildCommandTrie()
	}
	
	
	if len(words) == 1 && !strings.HasSuffix(lineStr, " ") {
		currentPrefix := strings.TrimSpace(prefix)
		
		
		if lastTabPrefix == currentPrefix {
			isTabPressed = true
		} else {
			isTabPressed = false
			completionState = nil
		}
		
		
		candidates := getMatchingCommands(currentPrefix)
		
		
		if len(candidates) == 0 {
			return [][]rune{[]rune("\a")}, 0
		} 
		
		
		if len(candidates) == 1 {
			completion := candidates[0][len(currentPrefix):] + " "
			lastTabPrefix = ""
			isTabPressed = false
			completionState = nil
			return [][]rune{[]rune(completion)}, len(prefix)
		}
		
		
		if currentPrefix == "" || len(candidates) > 1 {
			
			sort.Strings(candidates)
			fmt.Print("\n")
			fmt.Print(strings.Join(candidates, "  "))
			fmt.Print("\n")
			return [][]rune{[]rune("")}, 0
		}
		
		return [][]rune{[]rune("\a")}, 0
	}
	
	
	wordCount := len(words)
	if wordCount == 0 || (wordCount == 1 && !strings.HasSuffix(lineStr, " ")) {
		return completeCommand(words)
	} else {
		return completeArguement(lineStr, words)
	}
}

func findLongestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	
	prefix := strs[0]
	for i := 1; i < len(strs); i++ {
		j := 0
		for j < len(prefix) && j < len(strs[i]) && prefix[j] == strs[i][j] {
			j++
		}
		prefix = prefix[:j]
		if prefix == "" {
			return ""
		}
	}
	
	return prefix
}

func buildCommandTrie() *Trie {
	trie := NewTrie()
	
	
	for _, cmd := range builtinCommands {
		trie.Insert(cmd)
	}
	
	
	pathEnv := os.Getenv("PATH")
	paths := strings.Split(pathEnv, string(os.PathListSeparator))
	
	seen := make(map[string]bool)
	
	for _, dir := range paths {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		
		for _, entry := range entries {
			name := entry.Name()
			if !seen[name] {
				info, err := entry.Info()
				if err == nil && isExecutable(info.Mode()) {
					trie.Insert(name)
					seen[name] = true
				}
			}
		}
	}
	
	return trie
}

func completeCommand(words []string) ([][]rune, int) {
	prefix := ""
	if len(words) == 1 {
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
	
	seen := make(map[string]bool)
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
	
	if len(candidates) == 1 {
		completion := candidates[0][len(prefix):] + " "
		return [][]rune{[]rune(completion)}, len(prefix)
	} else {
		
		sort.Strings(candidates)
		fmt.Print("\n")
		fmt.Print(strings.Join(candidates, "  "))
		fmt.Print("\n")
		
		
		commonPrefix := findLongestCommonPrefix(candidates)
		if len(commonPrefix) > len(prefix) {
			completion := commonPrefix[len(prefix):]
			return [][]rune{[]rune(completion)}, len(prefix)
		}
		
		return [][]rune{[]rune("")}, 0
	}
}

func completeArguement(lineStr string, words []string) ([][]rune, int) {
	partial := ""
	if strings.HasSuffix(lineStr, " ") {
		partial = ""
	} else {
		partial = words[len(words)-1]
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
				candidates = append(candidates, fullPath[len(searchDir)-1:])
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
		
		commonPrefix := findLongestCommonPrefix(candidates)
		if len(commonPrefix) > len(prefix) {
			completion := commonPrefix[len(prefix):]
			return [][]rune{[]rune(completion)}, len(prefix)
		}
		
		
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