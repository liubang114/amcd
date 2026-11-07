package logx

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	mu  sync.Mutex
	out *os.File
)

// Init 打开 logs/<date>.log / open logs/<date>.log
func Init(baseDir string) error {
	dir := filepath.Join(baseDir, "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := time.Now().Format("2006-01-02") + ".log"
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	mu.Lock()
	out = f
	mu.Unlock()
	return nil
}

func write(level, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	line := fmt.Sprintf("[%s] [%s] %s", time.Now().Format("2006-01-02 15:04:05"), level, fmt.Sprintf(format, args...))
	if out != nil {
		_, _ = fmt.Fprintln(out, line)
	} else {
		_, _ = fmt.Println(line)
	}
}

func Info(format string, args ...any)  { write("INFO", format, args...) }
func Warn(format string, args ...any)  { write("WARN", format, args...) }
func Error(format string, args ...any) { write("ERROR", format, args...) }

// Close 关闭日志文件 / close the log file
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if out != nil {
		_ = out.Close()
		out = nil
	}
}
