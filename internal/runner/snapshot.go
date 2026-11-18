package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type snapshot struct {
	Directory string
	Digest    string
}

func copySource(ctx context.Context, root, destination string) (result snapshot, resultErr error) {
	output, err := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "-z").Output()
	if err != nil {
		return snapshot{}, fmt.Errorf("enumerate Git worktree: %w", err)
	}
	names := strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00")
	sort.Strings(names)
	directory, err := os.OpenRoot(root)
	if err != nil {
		return snapshot{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, directory.Close()) }()
	type entry struct {
		Path string
		Hash string
		Mode uint32
	}
	var manifest []entry
	seen := make(map[string]bool)
	for _, name := range names {
		if name == "" || seen[name] {
			continue
		}
		if err := ctx.Err(); err != nil {
			return snapshot{}, err
		}
		seen[name] = true
		if !filepath.IsLocal(name) {
			return snapshot{}, errors.New("worktree path escapes source root")
		}
		info, err := directory.Lstat(name)
		if errors.Is(err, os.ErrNotExist) {
			continue // A deleted tracked file must remain absent in the snapshot.
		}
		if err != nil {
			return snapshot{}, err
		}
		if !info.Mode().IsRegular() {
			return snapshot{}, fmt.Errorf("%s: only regular source files are supported; symlinks/submodules require assessment", name)
		}
		target := filepath.Join(destination, name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return snapshot{}, err
		}
		from, err := directory.Open(name)
		if err != nil {
			return snapshot{}, err
		}
		to, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return snapshot{}, errors.Join(err, from.Close())
		}
		hash := sha256.New()
		_, copyErr := io.Copy(io.MultiWriter(to, hash), from)
		if err := errors.Join(copyErr, from.Close(), to.Close()); err != nil {
			return snapshot{}, err
		}
		manifest = append(manifest, entry{Path: name, Hash: hex.EncodeToString(hash.Sum(nil)), Mode: uint32(info.Mode().Perm())})
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return snapshot{}, err
	}
	sum := sha256.Sum256(data)
	return snapshot{Directory: destination, Digest: "sha256:" + hex.EncodeToString(sum[:])}, nil
}
