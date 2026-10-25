package web

import (
	"errors"
	"io/fs"
	"sort"
)

// overlayFS serves files from primary and falls back to fallback for missing
// files. Directories are merged, with primary entries taking precedence.
type overlayFS struct {
	primary  fs.FS
	fallback fs.FS
}

func (o overlayFS) Open(name string) (fs.File, error) {
	file, err := o.primary.Open(name)
	if err == nil {
		return file, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return o.fallback.Open(name)
}

func (o overlayFS) ReadDir(name string) ([]fs.DirEntry, error) {
	primary, primaryErr := fs.ReadDir(o.primary, name)
	fallback, fallbackErr := fs.ReadDir(o.fallback, name)
	if primaryErr != nil && !errors.Is(primaryErr, fs.ErrNotExist) {
		return nil, primaryErr
	}
	if fallbackErr != nil && !errors.Is(fallbackErr, fs.ErrNotExist) {
		return nil, fallbackErr
	}
	if errors.Is(primaryErr, fs.ErrNotExist) && errors.Is(fallbackErr, fs.ErrNotExist) {
		return nil, fs.ErrNotExist
	}

	merged := make(map[string]fs.DirEntry, len(primary)+len(fallback))
	for _, entry := range fallback {
		merged[entry.Name()] = entry
	}
	for _, entry := range primary {
		merged[entry.Name()] = entry
	}
	entries := make([]fs.DirEntry, 0, len(merged))
	for _, entry := range merged {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}

func (o overlayFS) Stat(name string) (fs.FileInfo, error) {
	info, err := fs.Stat(o.primary, name)
	if err == nil {
		return info, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return fs.Stat(o.fallback, name)
}
