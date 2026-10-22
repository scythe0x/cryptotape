package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

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
	cmd := exec.Command("go", "build", "-tags", "production", "-trimpath", "-buildvcs=false", "-ldflags=-H windowsgui", "-o", "build/bin/Mythra.exe", ".")
	cmd.Env = environment(map[string]string{"GOOS": "windows", "GOARCH": "amd64", "CGO_ENABLED": "0"})
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func main() {
	if err := build(); err != nil {
		fmt.Fprintln(os.Stderr, "windows build failed:", err)
		os.Exit(1)
	}
}
