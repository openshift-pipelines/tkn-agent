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
		Link string `json:",omitempty"`
	}
	var manifest []entry
	seen := make(map[string]bool)
	available := make(map[string]bool)
	for _, name := range names {
		available[name] = true
	}
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
		target := filepath.Join(destination, name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return snapshot{}, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := directory.Readlink(name)
			if err != nil {
				return snapshot{}, err
			}
			relativeTarget := filepath.Clean(filepath.Join(filepath.Dir(name), link))
			resolved, err := filepath.EvalSymlinks(filepath.Join(root, name))
			if err != nil {
				return snapshot{}, fmt.Errorf("resolve source link %s: %w", name, err)
			}
			final, err := filepath.Rel(root, resolved)
			if err != nil || filepath.IsAbs(link) || !filepath.IsLocal(relativeTarget) ||
				!filepath.IsLocal(final) || !available[relativeTarget] || !available[final] {
				return snapshot{}, fmt.Errorf("%s: source link must stay within included repository files", name)
			}
			resolvedInfo, err := os.Stat(resolved)
			if err != nil || !resolvedInfo.Mode().IsRegular() {
				return snapshot{}, fmt.Errorf("%s: directory or unavailable source link is not supported", name)
			}
			if err := os.Symlink(link, target); err != nil {
				return snapshot{}, err
			}
			sum := sha256.Sum256([]byte("link:" + link))
			manifest = append(manifest, entry{Path: name, Hash: hex.EncodeToString(sum[:]), Mode: uint32(info.Mode().Perm()), Link: link})
			continue
		}
		if !info.Mode().IsRegular() {
			return snapshot{}, fmt.Errorf("%s: non-file source input is not supported", name)
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
