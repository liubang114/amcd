package cmd

import (
	"bufio"
	"io"
	"os"
	"strings"
)

// 全局 stdin 读取器：交互模式与询问共用，避免缓冲丢失
// shared stdin reader so prompts and the REPL do not fight over buffered input
var stdinReader = bufio.NewReader(os.Stdin)

// readLine 读取一行输入 / read one line from stdin
func readLine() (string, error) {
	line, err := stdinReader.ReadString('\n')
	line = strings.TrimSpace(line)
	if err != nil && line == "" {
		return "", err
	}
	if err == io.EOF {
		return line, io.EOF
	}
	return line, nil
}

// askYesNo 询问 y/n / ask a yes/no question
func askYesNo() bool {
	line, err := readLine()
	if err != nil && line == "" {
		return false
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}
