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
	Plan       Plan   `json:"plan"`
	Accepted   string `json:"accepted_digest"`
	SharedHash string `json:"shared_hash,omitempty"`
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
	shared, sharedHash, err := readShared(ctx, src)
	if err != nil {
		return Cache{}, false, err
	}
	path, err := cachePath(src)
	if err != nil {
		return Cache{}, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Cache{}, false, err
	}
	var cached Cache
	if len(data) > 0 {
		if err := decodeState(data, &cached); err != nil {
			return Cache{}, false, fmt.Errorf("invalid cached plan: %w", err)
		}
	}
	var sharedPlan Plan
	sharedCurrent := false
	if len(shared) > 0 {
		if err := decodeState(shared, &sharedPlan); err != nil {
			return Cache{}, false, fmt.Errorf("invalid shared plan; use --refresh to regenerate: %w", err)
		}
		if sharedPlan.Version != 2 {
			return Cache{}, false, errors.New("unsupported shared plan version; use --refresh")
		}
		sharedCurrent, err = currentPlan(ctx, src, sharedPlan, policy)
		if err != nil {
			return Cache{}, false, err
		}
	}
	if cached.SharedHash == sharedHash && cached.Plan.Version == 2 {
		current, err := currentPlan(ctx, src, cached.Plan, policy)
		if err != nil {
			return Cache{}, false, err
		}
		if current {
			return cached, true, nil
		}
	}
	if sharedCurrent {
		return Cache{Plan: sharedPlan, Accepted: cached.Accepted, SharedHash: sharedHash}, true, nil
	}
	return Cache{SharedHash: sharedHash}, false, nil
}

func currentPlan(ctx context.Context, src source.Inventory, p Plan, policy Policy) (bool, error) {
	if p.Version != 2 || p.Pipeline != src.Pipeline || p.Policy != policy.local() || p.Inputs[src.Pipeline] != src.Hash ||
		!reflect.DeepEqual(p.Parameters, src.Parameters) || !reflect.DeepEqual(p.Checkout, src.Checkout) {
		return false, nil
	}
	current, err := InputsCurrent(ctx, src.Root, p.Inputs)
	if err != nil || !current {
		return false, err
	}
	if len(p.Steps) != len(src.Steps) {
		return false, errors.New("saved plan changed source coverage")
	}
	proposal := struct {
		Steps []decision `json:"steps"`
	}{}
	for i, s := range p.Steps {
		if !reflect.DeepEqual(s.Source, src.Steps[i]) {
			return false, errors.New("saved execution differs from source; use --refresh to regenerate")
		}
		proposal.Steps = append(proposal.Steps, decision{ID: s.Source.ID, Decision: s.Decision, Reason: s.Reason})
	}
	selection, err := json.Marshal(proposal)
	if err != nil {
		return false, err
	}
	if _, err := Build(ctx, Request{Inventory: src, Selection: selection, Inputs: p.Inputs, Policy: policy}); err != nil {
		return false, err
	}
	return true, nil
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
