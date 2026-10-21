package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const backupFolder = "backups"

func Save(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}

	temporary, err := os.CreateTemp(directory, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())

	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}

	return os.Rename(temporary.Name(), path)
}

func Load(path string, value any) (string, error) {
	err := loadFile(path, value)
	if err == nil {
		return "", nil
	}
	if errors.Is(err, os.ErrNotExist) && len(Backups(path)) == 0 {
		return "", err
	}

	for _, backup := range Backups(path) {
		if loadFile(backup, value) == nil {
			return backup, nil
		}
	}

	return "", err
}

func loadFile(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("%s is damaged: %w", filepath.Base(path), err)
	}

	return nil
}

func Backups(path string) []string {
	prefix, extension := backupName(path)
	entries, err := os.ReadDir(filepath.Join(filepath.Dir(path), backupFolder))
	if err != nil {
		return nil
	}

	var backups []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasPrefix(name, prefix) && strings.HasSuffix(name, extension) {
			backups = append(backups, filepath.Join(filepath.Dir(path), backupFolder, name))
		}
	}

	slices.Sort(backups)
	slices.Reverse(backups)
	return backups
}

func backupName(path string) (string, string) {
	extension := filepath.Ext(path)
	return strings.TrimSuffix(filepath.Base(path), extension) + "-", extension
}

func Backup(path string, now time.Time, keep int) error {
	prefix, extension := backupName(path)
	directory := filepath.Join(filepath.Dir(path), backupFolder)
	target := filepath.Join(directory, prefix+now.Format("2006-01-02")+extension)

	if _, err := os.Stat(target); err == nil {
		return nil
	}

	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}

	source, err := os.Open(path)
	if err != nil {
		return err
	}
	defer source.Close()

	temporary, err := os.CreateTemp(directory, prefix+"*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())

	if _, err := io.Copy(temporary, source); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary.Name(), target); err != nil {
		return err
	}

	for _, old := range Backups(path)[min(keep, len(Backups(path))):] {
		os.Remove(old)
	}

	return nil
}

func SetAside(path string, now time.Time) (string, error) {
	extension := filepath.Ext(path)
	target := strings.TrimSuffix(path, extension) + ".damaged-" + now.Format("20060102-150405") + extension
	return target, os.Rename(path, target)
}
