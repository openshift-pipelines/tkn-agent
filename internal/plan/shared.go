package plan

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift-pipelines/tkn-agent/internal/source"
)

// SharedPath is relative to the repository, preserving each pipeline filename.
func SharedPath(src source.Inventory) string {
	for _, prefix := range []string{".tekton/", ".konflux/"} {
		if strings.HasPrefix(src.Pipeline, prefix) {
			return filepath.Join(strings.TrimSuffix(prefix, "/"), source.StateDirectory, strings.TrimPrefix(src.Pipeline, prefix)+".plan.json")
		}
	}
	return filepath.Join(source.StateDirectory, src.Pipeline+".plan.json")
}

func decodeState[T any](data []byte, value *T) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return errors.New("state contains trailing data")
	}
	return nil
}

func readShared(ctx context.Context, src source.Inventory) (data []byte, hash string, resultErr error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	root, err := os.OpenRoot(src.Root)
	if err != nil {
		return nil, "", err
	}
	defer func() { resultErr = errors.Join(resultErr, root.Close()) }()
	name := SharedPath(src)
	parts := strings.Split(filepath.ToSlash(name), "/")
	for i := range parts {
		info, err := root.Lstat(filepath.Join(parts[:i+1]...))
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", nil
		}
		if err != nil {
			return nil, "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, "", errors.New("shared plan paths cannot contain symlinks")
		}
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, "", err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return nil, "", err
	}
	if len(data) > 1<<20 {
		return nil, "", errors.New("shared plan exceeds 1 MiB")
	}
	sum := sha256.Sum256(data)
	return data, hex.EncodeToString(sum[:]), nil
}

// SharedHash permits explicit refresh without trusting the previous JSON.
func SharedHash(ctx context.Context, src source.Inventory) (string, error) {
	_, hash, err := readShared(ctx, src)
	return hash, err
}

// SaveShared persists only explicitly accepted plans; approval itself stays local.
func SaveShared(ctx context.Context, src source.Inventory, cached Cache) (result Cache, resultErr error) {
	digest, err := Digest(cached.Plan)
	if err != nil {
		return Cache{}, err
	}
	if cached.Accepted != digest {
		return Cache{}, errors.New("shared plan requires explicit acceptance")
	}
	current, err := InputsCurrent(ctx, src.Root, cached.Plan.Inputs)
	if err != nil || !current {
		return Cache{}, errors.Join(err, errors.New("planning inputs changed before saving the shared plan"))
	}
	root, err := os.OpenRoot(src.Root)
	if err != nil {
		return Cache{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, root.Close()) }()
	name := SharedPath(src)
	parts := strings.Split(filepath.ToSlash(filepath.Dir(name)), "/")
	for i := range parts {
		path := filepath.Join(parts[:i+1]...)
		if err := root.Mkdir(path, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
			return Cache{}, err
		}
		info, err := root.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return Cache{}, errors.New("shared plan parent must be a real directory")
		}
	}
	lockName := name + ".lock"
	lock, err := root.OpenFile(lockName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return Cache{}, fmt.Errorf("acquire shared plan write lock: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, lock.Close(), root.Remove(lockName)) }()
	data, err := json.MarshalIndent(cached.Plan, "", "  ")
	if err != nil {
		return Cache{}, err
	}
	data = append(data, '\n')
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	actual, err := SharedHash(ctx, src)
	if err != nil {
		return Cache{}, err
	}
	if actual != cached.SharedHash && actual != hash {
		return Cache{}, errors.New("shared plan changed during review; verify again")
	}
	if actual == hash {
		cached.SharedHash = hash
		return cached, nil
	}
	id := make([]byte, 12)
	if _, err := rand.Read(id); err != nil {
		return Cache{}, err
	}
	temp := filepath.Join(filepath.Dir(name), ".plan-"+hex.EncodeToString(id))
	file, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Cache{}, err
	}
	defer func() {
		if err := root.Remove(temp); err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	_, writeErr := file.Write(data)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return Cache{}, err
	}
	if err := root.Rename(temp, name); err != nil {
		return Cache{}, err
	}
	cached.SharedHash = hash
	return cached, nil
}
