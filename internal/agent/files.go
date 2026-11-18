package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

type repository struct {
	root   *os.File
	mu     sync.Mutex
	inputs map[string]string
	calls  int
	bytes  int
}

type readArgs struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line,omitempty"`
	MaxLines  int    `json:"max_lines,omitempty"`
}

type readResult struct {
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

func openRepository(directory string) (*repository, error) {
	file, err := os.Open(directory)
	if err != nil {
		return nil, fmt.Errorf("open planning root: %w", err)
	}
	return &repository{root: file, inputs: make(map[string]string)}, nil
}

// Each component is opened relative to a pinned directory descriptor, without
// following symlinks, including symlinks into credential directories in the root.
func (r *repository) open(name string) (*os.File, error) {
	if !safePath(name) {
		return nil, errors.New("read_file: forbidden repository path")
	}
	current := r.root
	for i, part := range strings.Split(name, "/") {
		flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK
		if i < len(strings.Split(name, "/"))-1 {
			flags |= unix.O_DIRECTORY
		}
		fd, err := unix.Openat(int(current.Fd()), part, flags, 0)
		if current != r.root {
			if closeErr := current.Close(); closeErr != nil {
				if err == nil {
					return nil, errors.Join(closeErr, unix.Close(fd))
				}
				return nil, errors.Join(err, closeErr)
			}
		}
		if err != nil {
			return nil, errors.New("read_file: unavailable path or symlink")
		}
		current = os.NewFile(uintptr(fd), part)
	}
	return current, nil
}

func (r *repository) read(ctx context.Context, args readArgs) (result readResult, resultErr error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.calls > 32 {
		return readResult{}, errors.New("read_file: 32 call limit exceeded")
	}
	if err := ctx.Err(); err != nil {
		return readResult{}, fmt.Errorf("read_file: %w", err)
	}
	if args.StartLine == 0 {
		args.StartLine = 1
	}
	if args.MaxLines == 0 {
		args.MaxLines = 200
	}
	if args.StartLine < 1 || args.MaxLines < 1 || args.MaxLines > 200 {
		return readResult{}, errors.New("read_file: start_line must be positive and max_lines within 1..200")
	}
	file, err := r.open(args.Path)
	if err != nil {
		return readResult{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return readResult{}, errors.New("read_file: only regular files are readable")
	}
	data, err := io.ReadAll(io.LimitReader(file, (64<<10)+1))
	if err != nil {
		return readResult{}, errors.New("read_file: cannot read file")
	}
	r.bytes += len(data)
	if len(data) > 64<<10 || r.bytes > 256<<10 {
		return readResult{}, errors.New("read_file: 64 KiB file or 256 KiB invocation limit exceeded")
	}
	if !utf8.Valid(data) || strings.ContainsRune(string(data), 0) || containsCredential(string(data)) {
		return readResult{}, errors.New("read_file: binary or possible credential material refused")
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	if previous, ok := r.inputs[args.Path]; ok && previous != hash {
		return readResult{}, errors.New("read_file: planning input changed during inspection")
	}
	r.inputs[args.Path] = hash
	lines := strings.Split(string(data), "\n")
	start := min(args.StartLine-1, len(lines))
	end := start + min(args.MaxLines, len(lines)-start)
	return readResult{Content: strings.Join(lines[start:end], "\n"), Truncated: start > 0 || end < len(lines)}, nil
}

func (r *repository) hashes() map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return maps.Clone(r.inputs)
}

func safePath(name string) bool {
	if name == "" || name == "." || strings.Contains(name, "\\") || path.IsAbs(name) || path.Clean(name) != name {
		return false
	}
	for _, part := range strings.Split(strings.ToLower(name), "/") {
		switch part {
		case "..", ".git", ".ssh", ".aws", ".azure", ".config", ".kube", ".docker",
			".npmrc", ".netrc", ".pypirc", ".git-credentials", "kubeconfig":
			return false
		}
		if strings.HasPrefix(part, ".env") || strings.Contains(part, "credential") ||
			strings.Contains(part, "secret") || strings.HasPrefix(part, "id_rsa") ||
			strings.HasPrefix(part, "id_ed25519") || strings.HasPrefix(part, "id_ecdsa") {
			return false
		}
		switch path.Ext(part) {
		case ".pem", ".key", ".p12", ".pfx", ".keystore":
			return false
		}
	}
	return true
}

var credentialPattern = regexp.MustCompile(`(?im)(-----BEGIN [^\r\n]*PRIVATE KEY-----|(?:password|passwd|api[_-]?key|access[_-]?token|auth[_-]?token|client[_-]?secret|authorization)\s*["']?\s*[:=]\s*["']?[^\s"'$<{\\]+|(?:gh[pousr]_|github_pat_|sk-ant-|AKIA)[A-Za-z0-9_-]{12,})`)

func containsCredential(content string) bool {
	if credentialPattern.MatchString(content) {
		return true
	}
	for _, name := range []string{"OPENAI_API_KEY", "GOOGLE_API_KEY", "GEMINI_API_KEY", "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN"} {
		if value := os.Getenv(name); value != "" && strings.Contains(content, value) {
			return true
		}
	}
	return false
}
