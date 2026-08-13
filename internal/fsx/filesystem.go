package fsx

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type FileSystem interface {
	ReadFile(string) ([]byte, error)
	ReadDir(string) ([]fs.DirEntry, error)
	Readlink(string) (string, error)
	Stat(string) (fs.FileInfo, error)
}

type OS struct{}

func (OS) ReadFile(name string) ([]byte, error)       { return os.ReadFile(name) }
func (OS) ReadDir(name string) ([]fs.DirEntry, error) { return os.ReadDir(name) }
func (OS) Readlink(name string) (string, error)       { return os.Readlink(name) }
func (OS) Stat(name string) (fs.FileInfo, error)      { return os.Stat(name) }

type Rooted struct{ Root string }

var encodedBDF = regexp.MustCompile(`^[0-9a-fA-F]{4}_[0-9a-fA-F]{2}_[0-9a-fA-F]{2}\.[0-7]$`)

func (r Rooted) resolve(name string) (string, error) {
	root, err := filepath.Abs(r.Root)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(strings.TrimLeft(filepath.ToSlash(name), "/"))
	// Fixture PCI BDF directory names are encoded with underscores so the
	// same fixture tree can be checked out on Windows. Decode/encode them
	// consistently on every host; otherwise Linux tests enumerate a decoded
	// name and then fail to resolve its fixture files.
	parts := strings.Split(filepath.ToSlash(clean), "/")
	for i, part := range parts {
		if strings.Count(part, ":") == 2 && strings.Contains(part, ".") {
			parts[i] = strings.ReplaceAll(part, ":", "_")
		}
	}
	clean = filepath.FromSlash(strings.Join(parts, "/"))
	if clean == "." {
		return root, nil
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("path escapes fixture root")
	}
	target := filepath.Join(root, filepath.FromSlash(clean))
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes fixture root")
	}
	return target, nil
}

func (r Rooted) ReadFile(name string) ([]byte, error) {
	p, err := r.resolve(name)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}
func (r Rooted) ReadDir(name string) ([]fs.DirEntry, error) {
	p, err := r.resolve(name)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}
	result := make([]fs.DirEntry, len(entries))
	for i, entry := range entries {
		if encodedBDF.MatchString(entry.Name()) {
			result[i] = renamedEntry{DirEntry: entry, name: strings.ReplaceAll(entry.Name(), "_", ":")}
		} else {
			result[i] = entry
		}
	}
	return result, nil
}

type renamedEntry struct {
	fs.DirEntry
	name string
}

func (e renamedEntry) Name() string { return e.name }
func (r Rooted) Readlink(name string) (string, error) {
	p, err := r.resolve(name)
	if err != nil {
		return "", err
	}
	return os.Readlink(p)
}
func (r Rooted) Stat(name string) (fs.FileInfo, error) {
	p, err := r.resolve(name)
	if err != nil {
		return nil, err
	}
	return os.Stat(p)
}
