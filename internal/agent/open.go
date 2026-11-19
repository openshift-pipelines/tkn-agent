package agent

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"golang.org/x/sys/unix"
)

type openedFile struct {
	file  *os.File
	name  string
	links map[string]string
}

// Final file links are resolved explicitly; credential/directory links never open.
func (r *repository) open(name string) (openedFile, error) {
	result := openedFile{name: name, links: make(map[string]string)}
	for range 8 {
		if !safePath(result.name) {
			return result, errors.New("read_file: forbidden repository path")
		}
		file, err := r.openRegular(result.name)
		if err == nil {
			result.file = file
			return result, nil
		}
		if !errors.Is(err, unix.ELOOP) {
			return result, err
		}
		if _, repeated := result.links[result.name]; repeated {
			return result, errors.New("read_file: symlink cycle refused")
		}
		target := make([]byte, 4096)
		n, err := unix.Readlinkat(int(r.root.Fd()), result.name, target)
		if err != nil || n == len(target) {
			return result, errors.New("read_file: symlink target unavailable")
		}
		raw := string(target[:n])
		if path.IsAbs(raw) {
			return result, errors.New("read_file: absolute symlink refused")
		}
		result.links[result.name] = "link:" + raw
		result.name = path.Clean(path.Join(path.Dir(result.name), raw))
	}
	return result, errors.New("read_file: symlink depth exceeded")
}

func (r *repository) openRegular(name string) (*os.File, error) {
	current := r.root
	parts := strings.Split(name, "/")
	for i, part := range parts {
		flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK
		if i < len(parts)-1 {
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
			if i < len(parts)-1 && (errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENOTDIR)) {
				return nil, errors.New("read_file: directory symlinks refused")
			}
			return nil, fmt.Errorf("read_file: unavailable path: %w", err)
		}
		current = os.NewFile(uintptr(fd), part)
	}
	return current, nil
}
