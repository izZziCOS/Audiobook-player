package testbooks

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var checked sync.Map

func ExpectFilesClosed(t *testing.T) {
	t.Helper()

	if _, already := checked.LoadOrStore(t, true); already {
		return
	}

	folder := filepath.Dir(t.TempDir())
	if resolved, err := filepath.EvalSymlinks(folder); err == nil {
		folder = resolved
	}

	t.Cleanup(func() {
		checked.Delete(t)
		for _, path := range openFiles() {
			if strings.HasPrefix(path, folder+string(filepath.Separator)) {
				t.Errorf("%s is still open when the test ends, which Windows won't allow to be deleted", path)
			}
		}
	})
}
