package sbctl

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func writeTarGz(t *testing.T, filename, entryName, contents string) {
	t.Helper()

	file, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: entryName,
		Mode: 0o600,
		Size: int64(len(contents)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write([]byte(contents)); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractBundleRejectsPathsOutsideOutputDirectory(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "out")
	if err := os.Mkdir(outputDir, 0o700); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		entryName string
		outside   string
	}{
		{
			name:      "parent traversal",
			entryName: "../outside.txt",
			outside:   filepath.Join(root, "outside.txt"),
		},
		{
			name:      "absolute path",
			entryName: filepath.Join(root, "absolute.txt"),
			outside:   filepath.Join(root, "absolute.txt"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archive := filepath.Join(root, "bundle-"+tt.name+".tgz")
			writeTarGz(t, archive, tt.entryName, "malicious content")

			if err := ExtractBundle(archive, outputDir); err == nil {
				t.Fatal("ExtractBundle() succeeded, want an error for an unsafe archive path")
			}
			if _, err := os.Stat(tt.outside); !os.IsNotExist(err) {
				t.Fatalf("outside file stat error = %v, want file not to exist", err)
			}
		})
	}
}

func TestExtractBundleExtractsLocalPath(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "bundle.tgz")
	outputDir := filepath.Join(root, "out")
	if err := os.Mkdir(outputDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTarGz(t, archive, "cluster-info/version.json", "bundle data")

	if err := ExtractBundle(archive, outputDir); err != nil {
		t.Fatalf("ExtractBundle() error = %v", err)
	}

	got, err := os.ReadFile(filepath.Join(outputDir, "cluster-info", "version.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "bundle data" {
		t.Fatalf("extracted contents = %q, want %q", got, "bundle data")
	}
}
