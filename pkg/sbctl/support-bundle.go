package sbctl

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

type ClusterData struct {
	ClusterInfoFile     string
	ClusterResourcesDir string
}

func ExtractBundle(filename string, outDir string) error {
	fileReader, err := os.Open(filename)
	if err != nil {
		return errors.Wrap(err, "failed to open input file")
	}

	gzf, err := gzip.NewReader(fileReader)
	if err != nil {
		return errors.Wrap(err, "failed to get new gzip reader")
	}

	tarReader := tar.NewReader(gzf)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create output directory")
	}
	outputRoot, err := os.OpenRoot(outDir)
	if err != nil {
		return errors.Wrap(err, "failed to open output directory")
	}
	defer outputRoot.Close()

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			return nil
		}

		if err != nil {
			return errors.Wrap(err, "failed to read tar header")
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}
		// Archive paths must be local before joining them to the output directory.
		// This rejects absolute paths and paths that escape via parent components.
		if !filepath.IsLocal(header.Name) || strings.Contains(header.Name, `\`) {
			return errors.Errorf("archive entry %q is not a local path", header.Name)
		}

		err = func() error {
			entryPath := filepath.Clean(header.Name)
			err := outputRoot.MkdirAll(filepath.Dir(entryPath), 0755)
			if err != nil {
				return errors.Wrap(err, "failed to create file path")
			}

			// Root-scoped operations prevent archive entries from escaping through
			// symlinked directories as well as through lexical path traversal.
			outFile, err := outputRoot.OpenFile(entryPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
			if err != nil {
				return errors.Wrap(err, "failed to create output file")
			}
			defer outFile.Close()

			// ignore decompression bombs
			_, err = io.Copy(outFile, tarReader) // nolint: gosec // ignore decompression bombs)
			if err != nil {
				return errors.Wrap(err, "failed to copy file")
			}

			return nil
		}()

		if err != nil {
			return err
		}
	}
}

func FindClusterData(bundlePath string) (ClusterData, error) {
	result := ClusterData{}

	err := filepath.Walk(bundlePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if info.Name() == "cluster-resources" {
				// Support bundle can have multiple cluster-resources directories.
				// We want the one at the root, so find the file with the shortest name
				if result.ClusterResourcesDir == "" || len(path) < len(result.ClusterResourcesDir) {
					result.ClusterResourcesDir = path
				}
			}
		} else if info.Name() == "cluster_version.json" {
			result.ClusterInfoFile = path
		}

		return nil
	})

	if err != nil {
		return result, errors.Wrap(err, "failed to walk bundle dir")
	}

	return result, nil
}
