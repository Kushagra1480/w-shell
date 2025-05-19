package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

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

func isBuiltinCommand(cmd string) bool {
    for _, builtinCmd := range builtinCommands {
        if cmd == builtinCmd {
            return true
        }
    }
    return false
}


func executeBuiltinCommand(cmd string, args []string) error {
    switch cmd {
    case "echo":
        
        
        fmt.Fprintln(os.Stdout, strings.Join(args, " "))
        
    case "type":
        if len(args) == 0 {
            fmt.Fprintln(os.Stdout, "type: missing argument")
            return fmt.Errorf("missing argument")
        }
        
        typeCommand := args[0]
        for _, builtinCmd := range builtinCommands {
            if builtinCmd == typeCommand {
                fmt.Fprintln(os.Stdout, typeCommand + " is a shell builtin")
                return nil
            }
        }
        
        execPath := findExecPath(typeCommand)
        if execPath != "" {
            fmt.Fprintln(os.Stdout, typeCommand + " is " + execPath)
        } else {
            fmt.Fprintln(os.Stdout, typeCommand + ": not found")
        }
        
    case "pwd":
        dir, err := os.Getwd()
        if err != nil {
            fmt.Fprintf(os.Stderr, "pwd: %v\n", err)
            return err
        }
        fmt.Fprintln(os.Stdout, dir)
        
    case "cd":
        
        if len(args) == 0 {
            return nil
        }
        
        dir := args[0]
        if dir == "~" {
            homeDir, err := os.UserHomeDir()
            if err != nil {
                fmt.Fprintf(os.Stderr, "cd: could not find home directory: %v\n", err)
                return err
            }
            dir = homeDir
        }
        
        err := os.Chdir(dir)
        if err != nil {
            fmt.Fprintf(os.Stderr, "cd: %s: No such file or directory\n", dir)
            return err
        }
        
    case "exit":
        os.Exit(0)
        
    default:
        return fmt.Errorf("unknown builtin command: %s", cmd)
    }
    
    return nil
}

func hasPipeline(input string) bool {
    var inSingleQuote, inDoubleQuote bool
    var escaped bool
    
    for i := 0; i < len(input); i++ {
        c := input[i]
        
        if escaped {
            escaped = false
            continue
        }
        
        if c == '\\' {
            escaped = true
            continue
        }
        
        if c == '\'' && !inDoubleQuote {
            inSingleQuote = !inSingleQuote
        } else if c == '"' && !inSingleQuote {
            inDoubleQuote = !inDoubleQuote
        } else if c == '|' && !inSingleQuote && !inDoubleQuote {
            return true
        }
    }
    
    return false
}


func splitByPipe(input string) []string {
    var result []string
    var current strings.Builder
    var inSingleQuote, inDoubleQuote bool
    var escaped bool
    
    for i := 0; i < len(input); i++ {
        c := input[i]
        
        if escaped {
            current.WriteByte(c)
            escaped = false
            continue
        }
        
        if c == '\\' {
            current.WriteByte(c)
            escaped = true
            continue
        }
        
        if c == '\'' && !inDoubleQuote {
            current.WriteByte(c)
            inSingleQuote = !inSingleQuote
        } else if c == '"' && !inSingleQuote {
            current.WriteByte(c)
            inDoubleQuote = !inDoubleQuote
        } else if c == '|' && !inSingleQuote && !inDoubleQuote {
            result = append(result, current.String())
            current.Reset()
        } else {
            current.WriteByte(c)
        }
    }
    
    if current.Len() > 0 {
        result = append(result, current.String())
    }
    
    return result
}

func executeMultiPipeline(commands []string) error {
    n := len(commands)
    if n < 2 {
        return fmt.Errorf("pipeline needs at least 2 commands")
    }
    
    
    pipes := make([][2]*os.File, n-1)
    for i := 0; i < n-1; i++ {
        pipeReader, pipeWriter, err := os.Pipe()
        if err != nil {
            for j := 0; j < i; j++ {
                pipes[j][0].Close()
                pipes[j][1].Close()
            }
            return fmt.Errorf("failed to create pipe: %v", err)
        }
        pipes[i][0] = pipeReader
        pipes[i][1] = pipeWriter
    }
    
    
    originalStdin := os.Stdin
    originalStdout := os.Stdout
    defer func() {
        os.Stdin = originalStdin
        os.Stdout = originalStdout
    }()
    
    var wg sync.WaitGroup
    
    
    errorCh := make(chan error, n)
    
    for i, cmdStr := range commands {
        cmdStr = strings.TrimSpace(cmdStr)
        cmdName, cmdArgs := parseCommand(cmdStr)
        
        
        var stdin *os.File
        if i == 0 {
            stdin = os.Stdin
        } else {
            stdin = pipes[i-1][0]
        }
        
        
        var stdout *os.File
        if i == n-1 {
            stdout = os.Stdout
        } else {
            stdout = pipes[i][1]
        }
        
        if isBuiltinCommand(cmdName) {
            
            
            
            if i == 0 {
                
                os.Stdin = stdin
                
                
                oldStdout := os.Stdout
                os.Stdout = stdout
                
                
                err := executeBuiltinCommand(cmdName, cmdArgs)
                
                
                os.Stdout = oldStdout
                
                
                stdout.Close()
                
                if err != nil {
                    cleanupPipes(pipes)
                    return fmt.Errorf("command %d failed: %v", i, err)
                }
            } else if i == n-1 {
                
                os.Stdin = stdin
                
                
                err := executeBuiltinCommand(cmdName, cmdArgs)
                
                if err != nil {
                    cleanupPipes(pipes)
                    return fmt.Errorf("command %d failed: %v", i, err)
                }
                
                
                stdin.Close()
            } else {
                
                tempStdin := os.Stdin
                tempStdout := os.Stdout
                
                os.Stdin = stdin
                os.Stdout = stdout
                
                
                err := executeBuiltinCommand(cmdName, cmdArgs)
                
                
                os.Stdin = tempStdin
                os.Stdout = tempStdout
                
                
                stdin.Close()
                stdout.Close()
                
                if err != nil {
                    cleanupPipes(pipes)
                    return fmt.Errorf("command %d failed: %v", i, err)
                }
            }
        } else {
            
            cmd := exec.Command(cmdName, cmdArgs...)
            cmd.Stdin = stdin
            cmd.Stdout = stdout
            cmd.Stderr = os.Stderr
            
            err := cmd.Start()
            if err != nil {
                cleanupPipes(pipes)
                return fmt.Errorf("failed to start command %d: %v", i, err)
            }
            
            
            if i > 0 {
                pipes[i-1][0].Close()
            }
            if i < n-1 {
                pipes[i][1].Close()
            }
            
            
            wg.Add(1)
            go func(cmd *exec.Cmd, index int) {
                defer wg.Done()
                err := cmd.Wait()
                if err != nil {
                    errorCh <- fmt.Errorf("command %d failed: %v", index, err)
                }
            }(cmd, i)
        }
    }
    
    
    wg.Wait()
    
    
    select {
    case err := <-errorCh:
        return err
    default:
        
    }
    
    return nil
}


func cleanupPipes(pipes [][2]*os.File) {
    for _, pipe := range pipes {
        if pipe[0] != nil {
            pipe[0].Close()
        }
        if pipe[1] != nil {
            pipe[1].Close()
        }
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

		if hasPipeline(line) {
			commands := splitByPipe(line)
			err := executeMultiPipeline(commands)
			if err != nil {
				fmt.Fprintf(os.Stderr, "pipeline error: %v\n", err)
			}
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
