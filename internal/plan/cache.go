package plan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/openshift-pipelines/tkn-agent/internal/source"
)

// Cache keeps acceptance separate from the executable plan digest.
type Cache struct {
	Plan     Plan   `json:"plan"`
	Accepted string `json:"accepted_digest"`
}

func cachePath(src source.Inventory) (string, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		var err error
		base, err = os.UserCacheDir()
		if err != nil {
			return "", err
		}
	}
	if !filepath.IsAbs(base) {
		return "", errors.New("cache directory must be absolute")
	}
	sum := sha256.Sum256([]byte(src.Root + "\x00" + src.Pipeline))
	return filepath.Join(base, "tkn-agent", "plans", hex.EncodeToString(sum[:])+".json"), nil
}

// Read revalidates source execution before allowing cached acceptance.
func Read(ctx context.Context, src source.Inventory, policy Policy) (Cache, bool, error) {
	path, err := cachePath(src)
	if err != nil {
		return Cache{}, false, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Cache{}, false, nil
	}
	if err != nil {
		return Cache{}, false, err
	}
	var cached Cache
	if err := json.Unmarshal(data, &cached); err != nil {
		return Cache{}, false, fmt.Errorf("invalid cached plan: %w", err)
	}
	p := cached.Plan
	if p.Version != 1 {
		return Cache{}, false, errors.New("unsupported cached plan version")
	}
	if p.Pipeline != src.Pipeline || p.Policy != policy.local() || p.Inputs[src.Pipeline] != src.Hash {
		return Cache{}, false, nil
	}
	current, err := InputsCurrent(ctx, src.Root, p.Inputs)
	if err != nil || !current {
		return Cache{}, false, err
	}
	if len(p.Steps) != len(src.Steps) {
		return Cache{}, false, errors.New("cached plan changed source coverage")
	}
	proposal := struct {
		Steps []decision `json:"steps"`
	}{}
	for i, s := range p.Steps {
		if !reflect.DeepEqual(s.Source, src.Steps[i]) {
			return Cache{}, false, errors.New("cached execution differs from source")
		}
		proposal.Steps = append(proposal.Steps, decision{ID: s.Source.ID, Decision: s.Decision, Reason: s.Reason})
	}
	selection, err := json.Marshal(proposal)
	if err != nil {
		return Cache{}, false, err
	}
	if _, err := Build(ctx, Request{Inventory: src, Selection: selection, Inputs: p.Inputs, Policy: policy}); err != nil {
		return Cache{}, false, err
	}
	return cached, true, nil
}

// Save atomically records a complete plan without modifying the source tree.
func Save(ctx context.Context, src source.Inventory, cached Cache) (resultErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := cachePath(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".plan-")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.Remove(file.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	_, writeErr := file.Write(data)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
