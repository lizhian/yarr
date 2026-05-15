package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultDBPathFromExecutable(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "yarr")

	if err := os.WriteFile(executable, nil, 0755); err != nil {
		t.Fatal(err)
	}

	got, err := defaultDBPathFromExecutable(executable)
	if err != nil {
		t.Fatal(err)
	}

	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(realDir, "storage.db")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDefaultDBPathFromExecutableResolvesSymlink(t *testing.T) {
	dir := t.TempDir()
	packageDir := filepath.Join(dir, "package")
	binDir := filepath.Join(dir, "bin")

	if err := os.Mkdir(packageDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(binDir, 0755); err != nil {
		t.Fatal(err)
	}

	executable := filepath.Join(packageDir, "yarr")
	if err := os.WriteFile(executable, nil, 0755); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(binDir, "yarr")
	if err := os.Symlink(executable, link); err != nil {
		t.Fatal(err)
	}

	got, err := defaultDBPathFromExecutable(link)
	if err != nil {
		t.Fatal(err)
	}

	realPackageDir, err := filepath.EvalSymlinks(packageDir)
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(realPackageDir, "storage.db")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
