package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const garbleVersion = "v0.17.0"

func command(env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
func environment(overrides map[string]string) []string {
	var result []string
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if _, replace := overrides[key]; !replace {
			result = append(result, item)
		}
	}
	for key, value := range overrides {
		result = append(result, key+"="+value)
	}
	return result
}
func build() error {
	tools, err := os.MkdirTemp("", "mythra-build-tools-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tools)
	root := runtime.GOROOT()
	// garble patches the linker through a go overlay; go rejects overlays beneath the module cache.
	if strings.Contains(root, string(filepath.Separator)+"pkg"+string(filepath.Separator)+"mod"+string(filepath.Separator)) {
		cache, cacheErr := os.UserCacheDir()
		if cacheErr != nil {
			return cacheErr
		}
		target := filepath.Join(cache, "mythra", "toolchains", runtime.Version()+"-"+runtime.GOOS+"-"+runtime.GOARCH)
		if _, statErr := os.Stat(filepath.Join(target, "bin", "go")); os.IsNotExist(statErr) {
			if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return err
			}
			temporary, copyErr := os.MkdirTemp(filepath.Dir(target), ".go-")
			if copyErr != nil {
				return copyErr
			}
			defer os.RemoveAll(temporary)
			if err = os.CopyFS(temporary, os.DirFS(root)); err != nil {
				return err
			}
			if err = os.Rename(temporary, target); err != nil {
				return err
			}
		}
		root = target
	}
	toolPath := filepath.Join(root, "bin") + string(os.PathListSeparator) + os.Getenv("PATH")
	hostEnv := environment(map[string]string{"GOOS": runtime.GOOS, "GOARCH": runtime.GOARCH, "GOBIN": tools, "GOROOT": root, "GOTOOLCHAIN": "local", "PATH": toolPath})
	if err = command(hostEnv, filepath.Join(root, "bin", "go"), "install", "mvdan.cc/garble@"+garbleVersion); err != nil {
		return err
	}
	raw := make([]byte, 8)
	if _, err = rand.Read(raw); err != nil {
		return err
	}
	seed := base64.RawStdEncoding.EncodeToString(raw)
	private := filepath.Join("build", "private", time.Now().UTC().Format("20060102T150405.000000000Z"))
	if err = os.MkdirAll(private, 0700); err != nil {
		return err
	}
	revision, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("read source revision: %w", err)
	}
	status, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return fmt.Errorf("read source status: %w", err)
	}
	metadata, err := json.MarshalIndent(struct {
		Go, Garble, Seed, Revision string
		Dirty                      bool
	}{runtime.Version(), garbleVersion, seed, strings.TrimSpace(string(revision)), len(status) > 0}, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(private, "build.json"), metadata, 0600); err != nil {
		return err
	}
	tool := filepath.Join(tools, "garble")
	if runtime.GOOS == "windows" {
		tool += ".exe"
	}
	env := environment(map[string]string{"GOOS": "windows", "GOARCH": "amd64", "CGO_ENABLED": "0", "GOGARBLE": "cryptotape/internal/*", "GOROOT": root, "GOTOOLCHAIN": "local", "PATH": toolPath})
	if err = command(env, tool, "-seed="+seed, "-literals", "build", "-tags", "production", "-trimpath", "-buildvcs=false", "-ldflags=-w -s -H windowsgui", "-o", "build/bin/Mythra.exe", "."); err != nil {
		return err
	}
	fmt.Println("protected executable: build/bin/Mythra.exe; private build metadata: " + private)
	return nil
}
func main() {
	if err := build(); err != nil {
		fmt.Fprintln(os.Stderr, "protected build failed:", err)
		os.Exit(1)
	}
}
