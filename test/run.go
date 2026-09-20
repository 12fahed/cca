//go:build ignore

// Command run executes the whole test suite: formatting, vet, build, unit
// tests, the race detector, and optionally the cross-compile and oracle checks.
//
// It is written in Go rather than shell so that it runs unchanged on Windows,
// where the project is expected to build and where a bash script would not run.
//
// Usage:
//
//	go run test/run.go             # everything except cross-compile
//	go run test/run.go -cross      # also build all five release targets
//	go run test/run.go -short      # skip the race detector
//	go run test/run.go -oracle DIR # also sweep a real Claude directory
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type step struct {
	name string
	args []string
	// env is appended to the environment for this step only.
	env []string
	// allowOutput marks a step whose output is meaningful even on success.
	allowOutput bool
	// failIfOutput marks a step that reports problems by printing rather than
	// by exit status, which is how gofmt behaves.
	failIfOutput bool
}

func main() {
	short := flag.Bool("short", false, "skip the race detector")
	cross := flag.Bool("cross", false, "also cross-compile every release target")
	oracle := flag.String("oracle", "", "sweep this Claude directory against cost-state records")
	flag.Parse()

	steps := []step{
		{name: "gofmt", args: []string{"gofmt", "-l", "."}, failIfOutput: true},
		{name: "vet", args: []string{"go", "vet", "./..."}},
		{name: "build", args: []string{"go", "build", "./..."}},
		{name: "test", args: []string{"go", "test", "./..."}, allowOutput: true},
	}
	if !*short {
		steps = append(steps, step{
			name: "race", args: []string{"go", "test", "-race", "./..."}, allowOutput: true,
		})
	}
	if *oracle != "" {
		steps = append(steps, step{
			name:        "oracle",
			args:        []string{"go", "test", "./internal/pricing/", "-run", "TestSweepAgainstReal", "-v", "-count=1"},
			env:         []string{"CCA_ORACLE_CLAUDE_DIR=" + *oracle},
			allowOutput: true,
		})
	}
	if *cross {
		for _, target := range []string{
			"darwin/arm64", "darwin/amd64", "linux/amd64", "linux/arm64", "windows/amd64",
		} {
			parts := strings.SplitN(target, "/", 2)
			steps = append(steps, step{
				name: "build " + target,
				args: []string{"go", "build", "-o", os.DevNull, "./cmd/cca"},
				env:  []string{"CGO_ENABLED=0", "GOOS=" + parts[0], "GOARCH=" + parts[1]},
			})
		}
	}

	var failed []string
	start := time.Now()
	for _, s := range steps {
		if !run(s) {
			failed = append(failed, s.name)
		}
	}

	fmt.Printf("\n%s\n", strings.Repeat("─", 60))
	if len(failed) > 0 {
		fmt.Printf("FAILED after %s: %s\n", time.Since(start).Round(time.Millisecond), strings.Join(failed, ", "))
		os.Exit(1)
	}
	fmt.Printf("all checks passed in %s\n", time.Since(start).Round(time.Millisecond))
}

func run(s step) bool {
	fmt.Printf("\n▸ %s\n", s.name)
	cmd := exec.Command(s.args[0], s.args[1:]...)
	cmd.Env = append(os.Environ(), s.env...)
	out, err := cmd.CombinedOutput()

	text := strings.TrimSpace(string(out))
	// gofmt reports problems by naming files, not by failing, so an empty
	// result is the only pass.
	if s.failIfOutput && text != "" {
		fmt.Printf("%s\nneeds formatting; run: gofmt -w .\n", text)
		return false
	}
	if err != nil {
		if text != "" {
			fmt.Println(text)
		}
		fmt.Printf("%v\n", err)
		return false
	}
	if s.allowOutput && text != "" {
		fmt.Println(indent(text))
	}
	return true
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}
