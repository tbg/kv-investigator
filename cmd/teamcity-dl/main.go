// teamcity-dl downloads build artifacts from TeamCity.
//
// Usage:
//
//	teamcity-dl <build-id> [output-dir]
//
// Environment:
//
//	TEAMCITY_TOKEN - Bearer token for authentication (required)
//	TEAMCITY_URL   - Base URL (default: https://teamcity.cockroachdb.com)
package main

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultBaseURL = "https://teamcity.cockroachdb.com"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: teamcity-dl <build-id> [output-dir]")
	}

	buildID := args[0]
	outDir := "."
	if len(args) >= 2 {
		outDir = args[1]
	}

	token := os.Getenv("TEAMCITY_TOKEN")
	if token == "" {
		return fmt.Errorf("TEAMCITY_TOKEN environment variable not set")
	}

	baseURL := os.Getenv("TEAMCITY_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	// Download all artifacts as a zip archive
	url := fmt.Sprintf("%s/app/rest/builds/id:%s/artifacts/archived", baseURL, buildID)

	fmt.Printf("Downloading artifacts for build %s...\n", buildID)

	zipPath := filepath.Join(outDir, fmt.Sprintf("build-%s-artifacts.zip", buildID))
	if err := download(url, token, zipPath); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	fmt.Printf("Downloaded: %s\n", zipPath)

	// Extract the zip
	extractDir := filepath.Join(outDir, fmt.Sprintf("build-%s", buildID))
	fmt.Printf("Extracting to: %s\n", extractDir)

	if err := extractZip(zipPath, extractDir); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Remove the zip file
	if err := os.Remove(zipPath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not remove zip file: %v\n", err)
	}

	// Count extracted files
	count := 0
	_ = filepath.Walk(extractDir, func(_ string, info os.FileInfo, _ error) error {
		if info != nil && !info.IsDir() {
			count++
		}
		return nil
	})

	fmt.Printf("Extracted %d files to %s\n", count, extractDir)
	return nil
}

func download(url, token, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Minute}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/zip")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	written, err := io.Copy(f, resp.Body)
	if err != nil {
		return err
	}

	fmt.Printf("Downloaded %.2f MB\n", float64(written)/(1024*1024))
	return nil
}

func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if err := extractZipFile(f, destDir); err != nil {
			return err
		}
	}
	return nil
}

func extractZipFile(f *zip.File, destDir string) error {
	// Sanitize path to prevent zip slip
	destPath := filepath.Join(destDir, f.Name)
	if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
		return fmt.Errorf("invalid file path: %s", f.Name)
	}

	if f.FileInfo().IsDir() {
		return os.MkdirAll(destPath, 0o755)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

