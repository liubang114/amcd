// Package downloader 并发下载、SHA-1 校验、原子落盘
// Package downloader provides concurrent downloads with SHA-1 verification
package downloader

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"amcd/i18n"
	"amcd/logx"
)

// File 一个待下载文件 / a file to download
type File struct {
	URL string
	// FallbackURL 镜像失效时的官方地址（可为空）/ official URL used when the mirror fails
	FallbackURL string
	Dest        string // 绝对路径 / absolute path
	SHA1        string
	Size        int64
}

// Options 下载选项 / download options
type Options struct {
	Workers int
	// AddBytes 进度回调（线程安全由实现方保证）/ progress callback
	AddBytes func(int)
	// OnSkip 跳过已完成文件 / called when a file is already valid
	OnSkip func(File)
	// OnError 记录单个失败文件 / record a failed file
	OnError func(File, error)
}

// Stats 统计 / result statistics
type Stats struct {
	Downloaded int
	Skipped    int
	Bytes      int64
}

var httpClient = &http.Client{
	Timeout: 0, // 大文件不设总超时 / no overall timeout for large files
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ResponseHeaderTimeout: 60 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConnsPerHost:   16,
		ForceAttemptHTTP2:     true,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

// Run 并发下载全部文件；单个文件失败不中断整轮，最后汇总报告
// Run downloads every file concurrently; individual failures are collected, not fatal
func Run(ctx context.Context, files []File, opts Options) (Stats, error) {
	var stats Stats
	if opts.Workers <= 0 {
		opts.Workers = 8
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan File)
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	statsMu := &sync.Mutex{}
	// 镜像连续失败后改用官方源优先 / switch to the official source after repeated mirror failures
	health := &mirrorHealth{}

	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				n, skipped, err := fetchOne(ctx, f, opts.AddBytes, health)
				if err != nil {
					mu.Lock()
					errs = append(errs, err)
					mu.Unlock()
					if opts.OnError != nil {
						opts.OnError(f, err)
					}
					continue
				}
				statsMu.Lock()
				stats.Bytes += n
				if skipped {
					stats.Skipped++
					if opts.OnSkip != nil {
						opts.OnSkip(f)
					}
				} else {
					stats.Downloaded++
				}
				statsMu.Unlock()
			}
		}()
	}

feed:
	for _, f := range files {
		select {
		case <-ctx.Done():
			break feed
		case jobs <- f:
		}
	}
	close(jobs)
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return stats, err
	}
	if len(errs) > 0 {
		head := errs[0]
		if len(errs) > 1 {
			return stats, fmt.Errorf("%w（共 %d 个文件失败）", head, len(errs))
		}
		return stats, head
	}
	return stats, nil
}

// mirrorHealth 记录镜像可用性 / tracks mirror reliability
type mirrorHealth struct {
	fails  atomic.Int32
	broken atomic.Bool
}

const mirrorFailThreshold = 6

func (h *mirrorHealth) mirrorFailed() {
	if h == nil {
		return
	}
	if h.fails.Add(1) >= mirrorFailThreshold {
		h.broken.Store(true)
	}
}

func (h *mirrorHealth) preferFallback() bool { return h != nil && h.broken.Load() }

// fetchOne 单个文件：优先跳过，失败重试，卡住自动断开
// fetchOne skips valid files, retries failures and aborts stalled transfers
func fetchOne(ctx context.Context, f File, addBytes func(int), health *mirrorHealth) (int64, bool, error) {
	if ok, size := valid(f); ok {
		if addBytes != nil && size > 0 {
			addBytes(int(size))
		}
		return size, true, nil
	}
	if err := os.MkdirAll(filepath.Dir(f.Dest), 0o755); err != nil {
		return 0, false, err
	}

	// 镜像优先；镜像失效时回退官方源，连续失败后整轮改为官方源优先
	urls := []string{f.URL}
	if f.FallbackURL != "" && f.FallbackURL != f.URL {
		if health.preferFallback() {
			urls = []string{f.FallbackURL, f.URL}
		} else {
			urls = append(urls, f.FallbackURL)
		}
	}

	var lastErr error
	for _, u := range urls {
		for attempt := 0; attempt < 2; attempt++ {
			n, err := transfer(ctx, f, u, addBytes)
			if err == nil {
				return n, false, nil
			}
			lastErr = err
			var se *statusError
			if errors.As(err, &se) && (se.code == 403 || se.code == 429) {
				// 限流不算镜像失效，退避后重试 / rate limiting is not a broken mirror
				select {
				case <-ctx.Done():
					return 0, false, lastErr
				case <-time.After(time.Duration(2*(attempt+1)) * time.Second):
				}
				continue
			}
			if u == f.URL && f.FallbackURL != "" && f.FallbackURL != f.URL {
				health.mirrorFailed()
			}
			if ctx.Err() != nil {
				return 0, false, lastErr
			}
			select {
			case <-ctx.Done():
				return 0, false, lastErr
			case <-time.After(time.Duration(attempt+1) * time.Second):
			}
		}
	}
	logx.Error("下载失败 %s: %v", f.Dest, lastErr)
	return 0, false, friendly(lastErr)
}

// bmclGate 限制 BMCLAPI 请求频率，避免 403/429 / throttle mirror requests
var bmclGate = struct {
	mu   sync.Mutex
	last time.Time
}{}

func pace(url string) {
	if !strings.Contains(url, "bmclapi") {
		return
	}
	bmclGate.mu.Lock()
	defer bmclGate.mu.Unlock()
	if d := time.Since(bmclGate.last); d < 40*time.Millisecond {
		time.Sleep(40*time.Millisecond - d)
	}
	bmclGate.last = time.Now()
}

func transfer(ctx context.Context, f File, url string, addBytes func(int)) (int64, error) {
	pace(url)
	fileCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	touch, stopWatch := watchStall(fileCtx, cancel, 15*time.Second)
	defer stopWatch()

	req, err := http.NewRequestWithContext(fileCtx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "amcd/0.1 (AL-1S Minecraft Downloader)")
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("下载失败 %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, &statusError{code: resp.StatusCode, url: url}
	}

	part := f.Dest + ".part"
	out, err := os.Create(part)
	if err != nil {
		return 0, err
	}

	var counted int
	report := func(n int) {
		counted += n
		if addBytes != nil {
			addBytes(n)
		}
	}

	hasher := sha1.New()
	writer := io.MultiWriter(out, hasher, writerFunc(func(p []byte) (int, error) {
		touch()
		report(len(p))
		return len(p), nil
	}))

	n, copyErr := io.Copy(writer, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		// 回退已计入的进度，避免进度条虚高 / roll back progress for this attempt
		if addBytes != nil && counted > 0 {
			addBytes(-counted)
		}
		// 用户中断时保留 .part，便于后续排查 / keep .part on interrupt
		if fileCtx.Err() == nil {
			_ = os.Remove(part)
		}
		return 0, fmt.Errorf("下载中断 %s: %w", f.Dest, copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(part)
		return 0, closeErr
	}

	if f.SHA1 != "" {
		got := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(got, f.SHA1) {
			_ = os.Remove(part)
			return 0, fmt.Errorf("SHA-1 校验失败 %s：期望 %s 实际 %s", f.Dest, strings.ToLower(f.SHA1), got)
		}
	}
	if err := os.Rename(part, f.Dest); err != nil {
		return 0, err
	}
	return n, nil
}

// statusError 非 200 响应 / a non-200 response
type statusError struct {
	code int
	url  string
}

func (e *statusError) Error() string { return fmt.Sprintf("HTTP %d %s", e.code, e.url) }

// friendly 把底层错误翻译成可读提示 / translate low-level errors
func friendly(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case errors.Is(err, syscall.ENOSPC) || strings.Contains(strings.ToLower(msg), "no space left"):
		return i18n.E("err.diskFull")
	case errors.Is(err, fs.ErrPermission) || strings.Contains(strings.ToLower(msg), "access is denied"):
		return i18n.E("err.permission")
	}
	return err
}

type writerFunc func([]byte) (int, error)

func (w writerFunc) Write(p []byte) (int, error) { return w(p) }

// watchStall 无数据 45 秒即中止本次传输 / abort a transfer that stalls
func watchStall(ctx context.Context, cancel context.CancelFunc, idle time.Duration) (touch func(), stop func()) {
	var last int64 = time.Now().UnixNano()
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if time.Since(time.Unix(0, atomic.LoadInt64(&last))) > idle {
					cancel()
					return
				}
			}
		}
	}()
	return func() { atomic.StoreInt64(&last, time.Now().UnixNano()) },
		func() { close(done) }
}

// valid 已存在且校验通过则跳过 / true when the destination is already correct
func valid(f File) (bool, int64) {
	info, err := os.Stat(f.Dest)
	if err != nil || info.IsDir() {
		return false, 0
	}
	if f.SHA1 == "" {
		if f.Size > 0 && info.Size() != f.Size {
			return false, 0
		}
		return true, info.Size()
	}
	sum, err := fileSHA1(f.Dest)
	if err != nil {
		return false, 0
	}
	if strings.EqualFold(sum, f.SHA1) {
		return true, info.Size()
	}
	return false, 0
}

func fileSHA1(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
