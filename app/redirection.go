package main

import (
	"regexp"
	"strings"
)

type RedirectionType int

const (
	RedirOut RedirectionType = iota
	RedirOutAppend
	RedirErr
	RedirErrAppend
)

type RedirPattern struct {
	Pattern *regexp.Regexp
	Type    RedirectionType
}

type ReDirection struct {
	Type     RedirectionType
	FilePath string
}

var redirectPatterns = []RedirPattern{
	{regexp.MustCompile(`(^|\s+)2>>(\s+|$)`), RedirErrAppend},
	{regexp.MustCompile(`(^|\s+)1>>(\s+|$)`), RedirOutAppend},
	{regexp.MustCompile(`(^|\s+)>>(\s+|$)`), RedirOutAppend},
	{regexp.MustCompile(`(^|\s+)2>(\s+|$)`), RedirErr},
	{regexp.MustCompile(`(^|\s+)1>(\s+|$)`), RedirOut},
	{regexp.MustCompile(`(^|\s+)>(\s+|$)`), RedirOut},
}

func extractRedirection(input string) (string, []ReDirection) {
    var redirections []ReDirection
	cmdString := input

	for _, pattern := range redirectPatterns {
		for {
			match := pattern.Pattern.FindStringIndex(cmdString)
			if match == nil {
				break
			}

			parts := strings.SplitN(cmdString[match[0]:], " ", 3)
			cmdString = cmdString[:match[0]]

			var filename string
			if len(parts) >= 2{
				filename = strings.TrimSpace(parts[len(parts) - 1])
			}

			redirections = append(redirections, ReDirection {
				Type: pattern.Type,
				FilePath: filename,
			})
		}
	}

	return strings.TrimSpace(cmdString), redirections
}