package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/chzyer/readline"
)
var builtinCommands = []string {
		"echo", "exit", "type", "pwd", "cd",
}
var _ = fmt.Fprint

var execPathCache = make(map[string]string)

func findExecPath(command string) string {

	if path, exists := execPathCache[command]; exists {
		return path
	}

	path, err := exec.LookPath(command)
	if err != nil {
		return ""
	}

	execPathCache[command] = path
	return path
}

func parseCommand(input string) (string, []string) {
	
	if input == "" {
		return "", []string{}
	}

	var result []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escapeNext := false

	for i := 0; i < len(input); i++ {
		char := rune(input[i])

		switch {
		
		case escapeNext:

			if inDoubleQuote || inSingleQuote {
				if char == '\\' || char == '"' || char == '$' {
					current.WriteRune(char)
				} else {
					current.WriteRune('\\')
					current.WriteRune(char)
				}
			} else {
				current.WriteRune(char)
			}
			escapeNext = false
		
		case char == '\\':
			if inSingleQuote {
				current.WriteRune('\\')
			} else {
				escapeNext = true
			}
			
		case char == '\'' && !inDoubleQuote:
			inSingleQuote = !inSingleQuote

		case char == '"' && !inSingleQuote:
			inDoubleQuote = !inDoubleQuote

		case char == ' ' && !inSingleQuote && !inDoubleQuote:
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}

		default:
			current.WriteRune(char)
		}		
	} 

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	if len(result) == 0 {
		return "",[]string{}
	}
	return result[0], result[1:]
}

func init() {
    logFile, err := os.OpenFile("/tmp/shell_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
    if err == nil {
        log.SetOutput(logFile)
    }
}

func main() {
	rl, err := readline.NewEx(&readline.Config {
		Prompt: "$ ",
		AutoComplete: &shellCompleter{},
		InterruptPrompt: "^C",
		EOFPrompt: "exit",
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	defer rl.Close()

	for {
		
		line, _ := rl.Readline()
		line = strings.TrimSpace((line))

		if line == "" {
			continue
		}
		cmdString, redirections := extractRedirection(line)
		commandName, args := parseCommand(cmdString)

		var originalStdout, originalStderr *os.File
		var stdoutFile, stderrFile *os.File

		for _, r := range redirections {
			var err error 
			var flags int

			switch r.Type {
			
			case RedirOut:
				flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
				originalStdout = os.Stdout
				stdoutFile, err = os.OpenFile(r.FilePath, flags, 0644)
				if err != nil {
					fmt.Fprintf(os.Stderr, "redirection error: %v\n", err)
					continue
				}
				os.Stdout = stdoutFile
				
				
			case RedirOutAppend:
				flags = os.O_CREATE | os.O_WRONLY | os.O_APPEND
				originalStdout = os.Stdout
				stdoutFile, err = os.OpenFile(r.FilePath, flags, 0644)
				if err != nil {
					fmt.Fprintf(os.Stderr, "redirection error: %v\n", err)
					continue
				}
				os.Stdout = stdoutFile
				
			case RedirErr:
				flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
				originalStderr = os.Stderr
				stderrFile, err = os.OpenFile(r.FilePath, flags, 0644)
				if err != nil {
					fmt.Fprintf(os.Stderr, "redirection error: %v\n", err)
					continue
				}
				os.Stderr = stderrFile
				
				
			case RedirErrAppend:
				flags = os.O_CREATE | os.O_WRONLY | os.O_APPEND
				originalStderr = os.Stderr
				stderrFile, err = os.OpenFile(r.FilePath, flags, 0644)
				if err != nil {
					fmt.Fprintf(os.Stderr, "redirection error: %v\n", err)
					continue
				}
				os.Stderr = stderrFile
			}
		} 

		switch commandName {
		
		case "exit":
			os.Exit(0)
		
		case "echo":

			fmt.Println(strings.Join(args, " "))

		case "type":
			
			if len(args) == 0 {
				fmt.Println("type: missing arguement")
				continue
			}

			typeCommand := args[0]
			isBuiltin := false

			for _, cmd := range builtinCommands {
				if cmd == typeCommand {
					isBuiltin = true
					break
				}
			}


			if isBuiltin {
				fmt.Println(typeCommand + " is a shell builtin")
			} else {
				execPath := findExecPath(typeCommand)
				if execPath != "" {
					fmt.Println(typeCommand + " is " +  execPath)
					continue
				} else {
					fmt.Println(typeCommand + ": not found")
				}
			}

		case "pwd":
			dir, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "pwd: %v\n", err)
			} else {
				fmt.Println(dir)
			}

		case "cd":
			if len(args) == 0 {
				continue
			}

			if args[0] == "~" {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					fmt.Fprintf(os.Stderr, "cd: could not find home directory: %v\n", err)
					continue
				}
				err = os.Chdir(homeDir)
				if err != nil {
					fmt.Fprintf(os.Stderr, "cd: %s: %v\n", homeDir, err)
				}
				continue
			}

			dir := args[0]
			err := os.Chdir(dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "cd: %s: No such file or directory\n", dir)
			}

		default:
			execPath := findExecPath(commandName)

			if execPath != "" {
				cmd := exec.Command(commandName, args...)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Run()
			} else {
				fmt.Println(commandName + ": command not found")
			}
		}
		if stdoutFile != nil {
			os.Stdout = originalStdout
			stdoutFile.Close()
		}
		if stderrFile != nil {
			os.Stderr = originalStderr
			stderrFile.Close()
		}
	}
}
