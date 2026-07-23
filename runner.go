package main

// this file runs three components of mechanical dinosaurs: the console, vm proxy, automation engine
//
// it is a standalone runner: no third-party dependencies, just the standard library. build it with
// `go build -o md-runner runner.go` and run the resulting binary from the repo root (the directory
// that contains the "frontend" folder and the "automation-engine" / "vm-proxy" binaries), or run it
// directly with `go run runner.go`.
//
// configuration is read from a ".env" file (see .env.example) plus a handful of MD_RUNNER_* variables
// that control how the runner itself behaves (see the "runner configuration" section below). anything
// already set in the process environment takes priority over the ".env" file.

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ---------------------------------------------------------------------------
// runner configuration
// ---------------------------------------------------------------------------
//
// these environment variables configure the runner itself. all of them are optional and have
// sensible defaults.
//
//   MD_ROOT                 working directory that contains "frontend", "automation-engine", and
//                           "vm-proxy" (default: the runner's own working directory)
//   MD_ENV_FILE             path to the env file to load (default: "<MD_ROOT>/.env")
//
//   CONSOLE_DIR             directory of the django project, relative to MD_ROOT (default: "frontend")
//   CONSOLE_CMD             full shell command used to start the console. overrides everything
//                           below. use this to switch to an ASGI server (uvicorn/daphne) or to a
//                           different WSGI server entirely.
//   CONSOLE_SERVER          "wsgi" or "asgi" (default: "wsgi")
//   CONSOLE_BIND            host:port to bind the console server to (default: "0.0.0.0:8000")
//   CONSOLE_WORKERS         number of worker processes for gunicorn/uvicorn (default: "3")
//   CONSOLE_PYTHON          python interpreter used to run manage.py and (if CONSOLE_MIGRATE is
//                           set) migrations (default: "python3")
//   CONSOLE_MIGRATE         if "true", run "manage.py migrate --noinput" before starting the server
//                           (default: "false")
//   CONSOLE_STATIC_DIR      directory where collected static files reside (default: "static")
//
//   AUTOMATION_ENGINE_BIN   path to the automation-engine binary, relative to MD_ROOT
//                           (default: "./automation-engine")
//   VM_PROXY_BIN            path to the vm-proxy binary, relative to MD_ROOT (default: "./vm-proxy")
//
//   MD_SHUTDOWN_GRACE       how long to wait after SIGTERM before force-killing a still-running
//                           component, e.g. "10s" (default: "10s")

// component is a single long running process managed by the runner.
type component struct {
	name string
	dir  string
	args []string // args[0] is the program to exec
}

func main() {
	root := getenv("MD_ROOT", mustGetwd())

	envFile := getenv("MD_ENV_FILE", filepath.Join(root, ".env"))
	fileEnv, err := loadEnvFile(envFile)
	if err != nil {
		if os.IsNotExist(err) {
			logf("runner", "no env file at %s, continuing with the process environment only", envFile)
		} else {
			fatalf("runner", "reading env file %s: %v", envFile, err)
		}
	}
	env := mergeEnv(os.Environ(), fileEnv)

	// Check for "run-migrations" subcommand
	if len(os.Args) > 1 && os.Args[1] == "run-migrations" {
		if err := executeRunMigrationsSubcommand(root, env); err != nil {
			fatalf("runner", "%v", err)
		}
		return
	}

	components, err := buildComponents(root, env)
	if err != nil {
		fatalf("runner", "%v", err)
	}

	grace := parseDurationOr(lookupEnv(env, "MD_SHUTDOWN_GRACE"), 10*time.Second)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := runAll(ctx, components, env, grace); err != nil {
		fatalf("runner", "%v", err)
	}
}

// executeRunMigrationsSubcommand compiles django SQL migrations and applies them to PostgreSQL via DATABASE_URL.
func executeRunMigrationsSubcommand(root string, env []string) error {
	logf("migrations", "starting compilation and execution of migrations")

	// 1. Run ./compile_django_sql.sh
	compileScript := filepath.Join(root, "compile_django_sql.sh")
	if _, err := os.Stat(compileScript); err != nil {
		return fmt.Errorf("compile script not found at %s: %w", compileScript, err)
	}

	logf("migrations", "running %s", compileScript)
	compileCmd := exec.Command(compileScript)
	compileCmd.Dir = root
	compileCmd.Env = env
	compileCmd.Stdout = newPrefixWriter(os.Stdout, "compile-sql")
	compileCmd.Stderr = newPrefixWriter(os.Stderr, "compile-sql")

	if err := compileCmd.Run(); err != nil {
		return fmt.Errorf("failed executing compile script: %w", err)
	}

	// 2. Validate DATABASE_URL exists in env
	dbURL := lookupEnv(env, "DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL is not set in environment or .env file")
	}

	// 3. Ensure all_migrations.sql exists
	sqlFile := filepath.Join(root, "all_migrations.sql")
	if _, err := os.Stat(sqlFile); err != nil {
		return fmt.Errorf("migration SQL file not found at %s: %w", sqlFile, err)
	}

	// 4. Run all_migrations.sql on postgres using psql
	logf("migrations", "applying %s to postgres database", sqlFile)
	psqlCmd := exec.Command("psql", dbURL, "-f", sqlFile)
	psqlCmd.Dir = root
	psqlCmd.Env = env
	psqlCmd.Stdout = newPrefixWriter(os.Stdout, "psql")
	psqlCmd.Stderr = newPrefixWriter(os.Stderr, "psql")

	if err := psqlCmd.Run(); err != nil {
		return fmt.Errorf("failed executing psql migration script: %w", err)
	}

	logf("migrations", "successfully completed migrations")
	return nil
}

// buildComponents assembles the three components (console, automation engine, vm proxy) using the
// merged environment for configuration.
func buildComponents(root string, env []string) ([]component, error) {
	consoleDir := filepath.Join(root, getenv("CONSOLE_DIR", "frontend"))
	if _, err := os.Stat(consoleDir); err != nil {
		return nil, fmt.Errorf("console directory %s: %w", consoleDir, err)
	}

	consoleArgs, err := consoleCommand(env, consoleDir)
	if err != nil {
		return nil, err
	}

	automationBin := absPath(root, getenv("AUTOMATION_ENGINE_BIN", "./automation-engine"))
	vmProxyBin := absPath(root, getenv("VM_PROXY_BIN", "./vm-proxy"))

	for _, bin := range []string{automationBin, vmProxyBin} {
		if _, err := os.Stat(bin); err != nil {
			return nil, fmt.Errorf("binary %s: %w", bin, err)
		}
	}

	return []component{
		{name: "console", dir: consoleDir, args: consoleArgs},
		{name: "automation-engine", dir: root, args: []string{automationBin}},
		{name: "vm-proxy", dir: root, args: []string{vmProxyBin}},
	}, nil
}

// consoleCommand builds the argv used to start the django console with WhiteNoise attached
// to serve static assets at /static.
func consoleCommand(env []string, consoleDir string) ([]string, error) {
	if custom := lookupEnv(env, "CONSOLE_CMD"); custom != "" {
		return []string{"sh", "-c", custom}, nil
	}

	bind := lookupEnvOr(env, "CONSOLE_BIND", "0.0.0.0:8000")
	workers := lookupEnvOr(env, "CONSOLE_WORKERS", "3")
	server := strings.ToLower(lookupEnvOr(env, "CONSOLE_SERVER", "wsgi"))

	staticDir := lookupEnvOr(env, "CONSOLE_STATIC_DIR", "static")
	if !filepath.IsAbs(staticDir) {
		staticDir = filepath.Join(consoleDir, staticDir)
	}

	python := lookupEnvOr(env, "CONSOLE_PYTHON", "python3")

	// We launch python directly to load WhiteNoise and wrap the application,
	// then hand off control directly to Gunicorn via python code.
	pyScript := fmt.Sprintf(`
import sys
from gunicorn.app.wsgiapp import WSGIApplication
from whitenoise import WhiteNoise

server = %q
static_dir = %q

if server == "asgi":
    from app.asgi import application as base_app
else:
    from app.wsgi import application as base_app

application = WhiteNoise(base_app, root=static_dir, prefix="/static/")

class StandaloneApplication(WSGIApplication):
    def load(self):
        return application

sys.argv = [
    "gunicorn",
    "--bind", %q,
    "--workers", %q,
]
if server == "asgi":
    sys.argv.extend(["-k", "uvicorn.workers.UvicornWorker"])

sys.argv.append("application")
StandaloneApplication().run()
`, server, staticDir, bind, workers)

	return []string{
		python, "-c", pyScript,
	}, nil
}

// runAll starts every component, streams their output with a name prefix, and blocks until either
// the context is cancelled (e.g. by SIGINT/SIGTERM) or any component exits, at which point every
// other component is asked to shut down as well.
func runAll(ctx context.Context, components []component, env []string, grace time.Duration) error {
	if migrate := lookupEnv(env, "CONSOLE_MIGRATE"); migrate == "true" || migrate == "1" {
		if err := runMigrations(components, env); err != nil {
			return fmt.Errorf("running console migrations: %w", err)
		}
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	errs := make(chan error, len(components))
	cmds := make([]*exec.Cmd, len(components))

	for i, c := range components {
		cmd := exec.Command(c.args[0], c.args[1:]...)
		cmd.Dir = c.dir
		cmd.Env = env
		cmd.Stdout = newPrefixWriter(os.Stdout, c.name)
		cmd.Stderr = newPrefixWriter(os.Stderr, c.name)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmds[i] = cmd

		if err := cmd.Start(); err != nil {
			cancel()
			return fmt.Errorf("starting %s: %w", c.name, err)
		}
		logf("runner", "started %s (pid %d)", c.name, cmd.Process.Pid)

		wg.Add(1)
		go func(name string, cmd *exec.Cmd) {
			defer wg.Done()
			err := cmd.Wait()
			if runCtx.Err() != nil {
				return
			}
			if err != nil {
				errs <- fmt.Errorf("%s exited: %w", name, err)
			} else {
				errs <- fmt.Errorf("%s exited unexpectedly with status 0", name)
			}
		}(c.name, cmd)
	}

	var runErr error
	select {
	case <-ctx.Done():
		logf("runner", "shutdown requested, stopping all components")
	case runErr = <-errs:
		logf("runner", "%v, stopping remaining components", runErr)
	}

	cancel()
	shutdownAll(cmds, grace)
	wg.Wait()

	return runErr
}

// runMigrations runs "manage.py migrate --noinput" in the console directory before the console
// server starts, using CONSOLE_PYTHON as the interpreter.
func runMigrations(components []component, env []string) error {
	var consoleDir string
	for _, c := range components {
		if c.name == "console" {
			consoleDir = c.dir
		}
	}
	if consoleDir == "" {
		return fmt.Errorf("no console component found")
	}

	python := lookupEnvOr(env, "CONSOLE_PYTHON", "python3")
	logf("runner", "running console migrations")

	cmd := exec.Command(python, "manage.py", "migrate", "--noinput")
	cmd.Dir = consoleDir
	cmd.Env = env
	cmd.Stdout = newPrefixWriter(os.Stdout, "console/migrate")
	cmd.Stderr = newPrefixWriter(os.Stderr, "console/migrate")
	return cmd.Run()
}

// shutdownAll sends SIGTERM to every still-running process group, waits up to grace for them to
// exit, then sends SIGKILL to anything left.
func shutdownAll(cmds []*exec.Cmd, grace time.Duration) {
	for _, cmd := range cmds {
		signalProcessGroup(cmd, syscall.SIGTERM)
	}

	done := make(chan struct{})
	go func() {
		for _, cmd := range cmds {
			if cmd.Process != nil {
				_, _ = cmd.Process.Wait()
			}
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(grace):
		logf("runner", "grace period elapsed, force killing remaining components")
		for _, cmd := range cmds {
			signalProcessGroup(cmd, syscall.SIGKILL)
		}
	}
}

func signalProcessGroup(cmd *exec.Cmd, sig syscall.Signal) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, sig)
}

// ---------------------------------------------------------------------------
// env file loading
// ---------------------------------------------------------------------------

func loadEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := splitEnvLine(line)
		if !ok {
			continue
		}
		result[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func splitEnvLine(line string) (key, value string, ok bool) {
	eq := strings.Index(line, "=")
	if eq < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:eq])
	rest := strings.TrimSpace(line[eq+1:])
	if key == "" {
		return "", "", false
	}

	if len(rest) > 0 && (rest[0] == '"' || rest[0] == '\'') {
		quote := rest[0]
		if end := strings.IndexByte(rest[1:], quote); end >= 0 {
			value = rest[1 : end+1]
			ok = true
			return key, value, ok
		}
	}

	if idx := strings.Index(rest, " #"); idx >= 0 {
		rest = strings.TrimSpace(rest[:idx])
	}
	return key, rest, true
}

func mergeEnv(base []string, file map[string]string) []string {
	present := make(map[string]bool, len(base))
	for _, kv := range base {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			present[kv[:i]] = true
		}
	}

	merged := append([]string{}, base...)
	for k, v := range file {
		if present[k] {
			continue
		}
		merged = append(merged, k+"="+v)
	}
	return merged
}

func lookupEnv(env []string, key string) string {
	prefix := key + "="
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], prefix) {
			return env[i][len(prefix):]
		}
	}
	return ""
}

func lookupEnvOr(env []string, key, def string) string {
	if v := lookupEnv(env, key); v != "" {
		return v
	}
	return def
}

// ---------------------------------------------------------------------------
// small helpers
// ---------------------------------------------------------------------------

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func absPath(root, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		fatalf("runner", "getting working directory: %v", err)
	}
	return wd
}

func parseDurationOr(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	return def
}

func logf(name, format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[%s] %s\n", name, fmt.Sprintf(format, args...))
}

func fatalf(name, format string, args ...any) {
	logf(name, format, args...)
	os.Exit(1)
}

type prefixWriter struct {
	mu     sync.Mutex
	out    io.Writer
	prefix string
	buf    strings.Builder
}

func newPrefixWriter(out io.Writer, name string) *prefixWriter {
	return &prefixWriter{out: out, prefix: "[" + name + "] "}
}

func (w *prefixWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	n := len(p)
	for len(p) > 0 {
		idx := strings.IndexByte(string(p), '\n')
		if idx < 0 {
			w.buf.Write(p)
			break
		}
		w.buf.Write(p[:idx])
		fmt.Fprintf(w.out, "%s%s\n", w.prefix, w.buf.String())
		w.buf.Reset()
		p = p[idx+1:]
	}
	return n, nil
}
