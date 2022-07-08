package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func defaultSdkFolder() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Android", "sdk")
	case "windows":
		return filepath.Join(home, "AppData", "Local", "Android", "Sdk")
	case "linux":
		return filepath.Join(home, "Android", "Sdk")
	default:
		return ""
	}
}

func findADB() (string, error) {
	// Look for adb in PATH
	path, err := exec.LookPath("adb")
	if err == nil {
		return path, nil
	}

	// Look for adb in default Android Studio location
	switch runtime.GOOS {
	case "windows":
		path = filepath.Join(defaultSdkFolder(), "platform-tools", "adb.exe")
	default:
		path = filepath.Join(defaultSdkFolder(), "platform-tools", "adb")
	}
	if _, err = os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return "", os.ErrNotExist
	}
	return path, nil
}
