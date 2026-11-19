package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestReadFileRejectsEscapesCredentialsAndSymlinks(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, ".env.local", "PASSWORD=do-not-disclose")
	writeFixture(t, root, ".git/config", "do-not-disclose")
	writeFixture(t, root, "config.yaml", "password: do-not-disclose")
	writeFixture(t, root, "private.pem", "do-not-disclose")
	writeFixture(t, root, "huge.txt", strings.Repeat("x", (64<<10)+1))
	if err := os.Symlink(".env.local", filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(".git", filepath.Join(root, "aliasdir")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "outside")); err != nil {
		t.Fatal(err)
	}
	files, err := openRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := files.root.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, name := range []string{
		"../outside", "/etc/passwd", ".env.local", ".git/config", "private.pem",
		"alias", "aliasdir/config", "outside/file", "config.yaml", "huge.txt", ".",
	} {
		t.Run(name, func(t *testing.T) {
			result, err := files.read(t.Context(), readArgs{Path: name})
			if err == nil || result.Content != "" {
				t.Fatalf("unsafe read %q disclosed data: %+v, %v", name, result, err)
			}
		})
	}
	if hashes := files.hashes(); len(hashes) != 0 {
		t.Fatalf("denied reads recorded as planning inputs: %v", hashes)
	}
}

func TestReadFileHashesFullInputUnderConcurrentBoundedReads(t *testing.T) {
	root := t.TempDir()
	files, err := openRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := files.root.Close(); err != nil {
			t.Error(err)
		}
	})
	want := make(map[string]string)
	for i := range 8 {
		name := fmt.Sprintf("helper%d.sh", i)
		content := fmt.Sprintf("echo %d\necho end\n", i)
		writeFixture(t, root, name, content)
		sum := sha256.Sum256([]byte(content))
		want[name] = hex.EncodeToString(sum[:])
	}
	var group sync.WaitGroup
	for name := range want {
		group.Go(func() {
			result, err := files.read(t.Context(), readArgs{Path: name, MaxLines: 1})
			if err != nil || !result.Truncated || strings.Contains(result.Content, "end") {
				t.Errorf("bounded read: %+v %v", result, err)
			}
		})
	}
	group.Wait()
	got := files.hashes()
	if len(got) != len(want) {
		t.Fatalf("lost concurrent inputs: %v", got)
	}
	for name, hash := range want {
		if got[name] != hash {
			t.Errorf("hash %s = %s; want %s", name, got[name], hash)
		}
	}
}

func TestReadFileRejectsChangedPlanningInput(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "Makefile", "check:\n\ttrue\n")
	files, err := openRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := files.root.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := files.read(t.Context(), readArgs{Path: "Makefile"}); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "Makefile", "check:\n\tfalse\n")

	if _, err := files.read(t.Context(), readArgs{Path: "Makefile"}); err == nil {
		t.Fatal("changed input accepted within one planning session")
	}
}

func TestReadFilePreservesSafeLinkAndOptionalAbsenceInputs(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "checks.conf", "enabled=true\n")
	if err := os.Symlink("checks.conf", filepath.Join(root, "check-link")); err != nil {
		t.Fatal(err)
	}
	files, err := openRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := files.root.Close(); err != nil {
			t.Error(err)
		}
	})
	got, err := files.read(t.Context(), readArgs{Path: "check-link"})
	if err != nil || !got.Exists || got.Content != "enabled=true\n" {
		t.Fatalf("safe input link not readable: %+v %v", got, err)
	}
	missing, err := files.read(t.Context(), readArgs{Path: "optional.conf"})
	if err != nil || missing.Exists {
		t.Fatalf("optional absence not represented: %+v %v", missing, err)
	}
	sum := sha256.Sum256([]byte("enabled=true\n"))
	hashes := files.hashes()
	if len(hashes) != 3 || hashes["check-link"] != "link:checks.conf" ||
		hashes["checks.conf"] != hex.EncodeToString(sum[:]) || hashes["optional.conf"] != "absent" {
		t.Fatalf("planning input provenance incomplete: %+v", hashes)
	}
}
