package downcan

import (
	"archive/zip"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
)

var opts struct {
	LogFormat     string `long:"log-format" choice:"text" choice:"json" default:"text" description:"Log format"`
	Verbose       []bool `short:"v" long:"verbose" description:"Show verbose debug information, each -v bumps log level"`
	logLevel      slog.Level
	DataDirectory string `short:"d" long:"data-dir" description:"Directory to recursively search for zip files"`
}

func Execute() int {
	if err := parseFlags(); err != nil {
		return 1
	}

	if err := setLogLevel(); err != nil {
		slog.Error("error setting log level", "error", err)
		return 1
	}

	if err := setupLogger(); err != nil {
		slog.Error("error setting up logger", "error", err)
		return 1
	}

	if err := run(); err != nil {
		slog.Error("run failed", "error", err)
		return 1
	}

	return 0
}

func parseFlags() error {
	_, err := flags.Parse(&opts)
	return err
}

func run() error {
	slog.Info("starting", "opts", opts)

	if opts.DataDirectory == "" {
		return fmt.Errorf("please provide a data directory using the --data-dir flag")
	}

	zipFiles, err := findZipFiles(opts.DataDirectory)
	if err != nil {
		return fmt.Errorf("error finding zip files: %w", err)
	}

	slog.Debug("found zip files", "count", len(zipFiles), "files", zipFiles)

	for _, zipFile := range zipFiles {
		destDir := getExpandedPath(zipFile)

		if _, err := os.Stat(destDir); err == nil {
			slog.Info("skipping expanding since target exists", "zip", zipFile, "destDir", destDir)
			continue
		}

		err := os.MkdirAll(destDir, os.ModePerm)
		if err != nil {
			slog.Error("error creating directory", "destDir", destDir, "error", err)
			continue
		}

		err = extractZipFile(zipFile, destDir)
		if err != nil {
			slog.Error("error extracting", "zipFile", zipFile, "error", err)
			continue
		}
	}

	return nil
}

func findZipFiles(directory string) ([]string, error) {
	var zipFiles []string

	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking %s: %w", path, err)
		}

		if info.IsDir() {
			return nil
		}

		contentType, err := getFileContentType(path)
		if err != nil {
			slog.Error("error getting content type", "path", path, "error", err)
		}

		if contentType == "application/zip" {
			zipFiles = append(zipFiles, path)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking %s: %w", directory, err)
	}

	return zipFiles, nil
}

func getFileContentType(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening %s: %w", filePath, err)
	}
	defer file.Close()

	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		return "", fmt.Errorf("error reading %s: %w", filePath, err)
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return "", fmt.Errorf("error seeking %s: %w", filePath, err)
	}

	return http.DetectContentType(buffer), nil
}

func extractZipFile(zipFilePath, destDir string) error {
	r, err := zip.OpenReader(zipFilePath)
	if err != nil {
		return fmt.Errorf("error opening %s: %w", zipFilePath, err)
	}
	defer r.Close()

	for _, file := range r.File {
		target := filepath.Join(destDir, file.Name)

		if file.FileInfo().IsDir() {
			err := os.MkdirAll(target, os.ModePerm)
			if err != nil {
				return fmt.Errorf("error creating directory %s: %w", target, err)
			}
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("error opening %s: %w", file.Name, err)
		}
		defer rc.Close()

		f, err := os.Create(target)
		if err != nil {
			return fmt.Errorf("error creating %s: %w", target, err)
		}
		defer f.Close()

		_, err = io.Copy(f, rc)
		if err != nil {
			return fmt.Errorf("error copying %s: %w", target, err)
		}

		slog.Debug("extracted file", "zip", zipFilePath, "file", target)
	}

	return nil
}

func getExpandedPath(zipFilePath string) string {
	baseDir := filepath.Dir(zipFilePath)
	zipFileName := filepath.Base(zipFilePath)
	expandedDir := filepath.Join(baseDir, "expanded", zipFileName[:len(zipFileName)-4]) // Removing ".zip" extension
	return expandedDir
}
