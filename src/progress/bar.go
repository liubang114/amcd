// Package progress 天蓝色单行进度条 / sky-blue single-line progress bar
package progress

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/term"
)

// 天蓝色 #87CEEB / sky blue
var (
	sky   = "\x1b[38;2;135;206;235m"
	reset = "\x1b[0m"
)

// SetColorEnabled 关闭颜色时进度条退回纯文本/无底色
// SetColorEnabled disables ANSI colors in the bar
func SetColorEnabled(on bool) {
	if on {
		sky = "\x1b[38;2;135;206;235m"
		reset = "\x1b[0m"
		return
	}
	sky, reset = "", ""
}

// maxWidth 单行最大可见宽度上限，超过会折行造成空白 / keep one screen line
const maxWidth = 78

// Bar 线程安全的单行进度条 / thread-safe single-line progress bar
type Bar struct {
	total int64
	cur   atomic.Int64
	desc  string
	start time.Time

	mu       sync.Mutex
	maxW     int
	width    int
	lastLen  int
	rendered int
	finished bool

	stop     chan struct{}
	done     chan struct{}
	isTerm   bool
	indet    bool
	disabled bool
}

// New 创建并启动进度条 / create and start a progress bar
func New(total int64, desc string) *Bar {
	b := &Bar{
		total:  total,
		desc:   desc,
		start:  time.Now(),
		width:  20,
		maxW:   maxWidth,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
		isTerm: term.IsTerminal(int(os.Stdout.Fd())),
	}
	if b.total <= 0 {
		// 总量未知（例如安装器 jar）：只显示已下载量与速度
		b.indet = true
		b.total = 1
	}
	if b.isTerm {
		if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 20 {
			if w-1 < b.maxW {
				b.maxW = w - 1
			}
		}
	}
	go b.loop()
	return b
}

func (b *Bar) loop() {
	defer close(b.done)
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-b.stop:
			return
		case <-ticker.C:
			b.render(false)
		}
	}
}

// Add 推进进度 / advance the bar
func (b *Bar) Add(n int) error {
	if b == nil {
		return nil
	}
	b.cur.Add(int64(n))
	return nil
}

// Finish 结束并换行 / finish and move to the next line
func (b *Bar) Finish() {
	if b == nil {
		return
	}
	b.mu.Lock()
	if b.finished {
		b.mu.Unlock()
		return
	}
	b.finished = true
	b.mu.Unlock()

	close(b.stop)
	<-b.done
	b.render(true)
}

func (b *Bar) render(final bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	cur := b.cur.Load()
	if cur < 0 {
		cur = 0
	}
	if !b.indet && cur > b.total {
		cur = b.total
	}
	pct := float64(cur) / float64(b.total) * 100

	filled := int(pct / 100 * float64(b.width))
	if filled > b.width {
		filled = b.width
	}
	bar := sky + strings.Repeat("█", filled) + reset +
		strings.Repeat(" ", b.width-filled)

	elapsed := time.Since(b.start).Seconds()
	speed := ""
	if elapsed > 0.3 {
		speed = fmt.Sprintf(" %s/s", humanBytes(float64(cur)/elapsed))
	}
	line := fmt.Sprintf("%s %3.0f%% %s %s%s", pad(b.desc, 14), pct, bar, humanBytes(float64(cur)), speed)
	if b.indet {
		line = fmt.Sprintf("%s %s%s", pad(b.desc, 14), humanBytes(float64(cur)), speed)
	}

	visible := stripANSI(line)
	if len([]rune(visible)) > b.maxW {
		line = truncateVisible(line, b.maxW)
		visible = stripANSI(line)
	}

	if !b.isTerm {
		// 非终端（重定向）时按 10% 打点，避免日志被覆盖 / log-friendly output when piped
		if final || (!b.indet && (pct >= float64(b.rendered+10) || pct >= 100)) {
			fmt.Fprintln(os.Stdout, visible)
			b.rendered = int(pct / 10 * 10)
		}
		return
	}

	padN := b.lastLen - len([]rune(visible))
	if padN < 0 {
		padN = 0
	}
	fmt.Fprintf(os.Stdout, "\r%s%s", line, strings.Repeat(" ", padN))
	b.lastLen = len([]rune(visible))
	if final {
		fmt.Fprint(os.Stdout, "\n")
	}
}

func pad(s string, n int) string {
	r := []rune(s)
	if len(r) >= n {
		return string(r[:n])
	}
	return s + strings.Repeat(" ", n-len(r))
}

func humanBytes(n float64) string {
	units := []string{"B", "KB", "MB", "GB"}
	i := 0
	for n >= 1024 && i < len(units)-1 {
		n /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f%s", n, units[i])
	}
	return fmt.Sprintf("%.1f%s", n, units[i])
}

func stripANSI(s string) string {
	var out strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			inEsc = true
		case inEsc && r == 'm':
			inEsc = false
		case !inEsc:
			out.WriteRune(r)
		}
	}
	return out.String()
}

// truncateVisible 按可见宽度截断，保留 ANSI 序列 / truncate by visible width
func truncateVisible(s string, width int) string {
	var out strings.Builder
	count := 0
	inEsc := false
	for _, r := range s {
		if r == 0x1b {
			inEsc = true
			out.WriteRune(r)
			continue
		}
		if inEsc {
			out.WriteRune(r)
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		if count >= width {
			break
		}
		out.WriteRune(r)
		count++
	}
	return out.String() + reset
}
