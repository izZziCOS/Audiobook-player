//go:build !darwin && !linux

package testbooks

func openFiles() []string {
	return nil
}
