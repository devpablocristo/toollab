package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Executor struct {
	coreDir    string
	binaryPath string
}

func New(coreDir string) *Executor {
	e := &Executor{coreDir: coreDir}
	e.binaryPath = e.findBinary()
	return e
}

func (e *Executor) findBinary() string {
	candidates := []string{
		filepath.Join(filepath.Dir(os.Args[0]), "toollab"),
		"/app/toollab",
		filepath.Join(e.coreDir, "toollab"),
		"toollab",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func (e *Executor) ensureBinary() (string, error) {
	if e.binaryPath != "" {
		if _, err := os.Stat(e.binaryPath); err == nil {
			return e.binaryPath, nil
		}
	}

	goPath, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("toollab binary not found and go compiler not available — rebuild the Docker image")
	}

	bin := filepath.Join(e.coreDir, "toollab")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, goPath, "build", "-o", bin, "./cmd/toollab")
	cmd.Dir = e.coreDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("build toollab: %s: %w", string(out), err)
	}
	e.binaryPath = bin
	return bin, nil
}

type Result struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
	Elapsed string `json:"elapsed"`
}

func (e *Executor) Run(ctx context.Context, args ...string) Result {
	start := time.Now()

	bin, err := e.ensureBinary()
	if err != nil {
		return Result{Error: err.Error(), Elapsed: time.Since(start).String()}
	}

	args = rewriteLocalhostArgs(args)

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = e.coreDir

	envFile := filepath.Join(e.coreDir, ".env")
	cmd.Env = os.Environ()
	if _, err := os.Stat(envFile); err == nil {
		cmd.Env = append(cmd.Env, loadEnvPairs(envFile)...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return Result{
			Output:  stdout.String(),
			Error:   stderr.String() + "\n" + err.Error(),
			Elapsed: time.Since(start).String(),
		}
	}

	return Result{
		Success: true,
		Output:  stdout.String(),
		Elapsed: time.Since(start).String(),
	}
}

func (e *Executor) CoreDir() string { return e.coreDir }

func inDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

func rewriteLocalhostArgs(args []string) []string {
	if !inDocker() {
		return args
	}
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = strings.ReplaceAll(
			strings.ReplaceAll(a, "://localhost:", "://host.docker.internal:"),
			"://127.0.0.1:", "://host.docker.internal:",
		)
	}
	return out
}

func loadEnvPairs(path string) []string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	docker := inDocker()
	existing := envSet()
	var pairs []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, _, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if existing[key] {
			continue
		}
		if docker {
			line = strings.ReplaceAll(
				strings.ReplaceAll(line, "://localhost:", "://host.docker.internal:"),
				"://127.0.0.1:", "://host.docker.internal:",
			)
		}
		pairs = append(pairs, line)
	}
	return pairs
}

func envSet() map[string]bool {
	m := make(map[string]bool)
	for _, e := range os.Environ() {
		if k, _, ok := strings.Cut(e, "="); ok {
			m[k] = true
		}
	}
	return m
}
