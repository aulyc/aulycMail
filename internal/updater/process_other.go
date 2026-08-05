//go:build !darwin

package updater

func processAlive(int) bool { return false }
