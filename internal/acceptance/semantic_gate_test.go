package acceptance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kachofugetsu09/nemeton/internal/api"
	"github.com/kachofugetsu09/nemeton/internal/event"
	_ "modernc.org/sqlite"
)

var testBinaries string

func TestMain(m *testing.M) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		fmt.Fprintf(os.Stderr, "semantic gate requires Linux or macOS, got %s\n", runtime.GOOS)
		os.Exit(1)
	}
	root, err := repositoryRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	testBinaries, err = os.MkdirTemp("", "nemeton-semantic-gate-binaries-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, command := range []string{"nemeton", "nemetond"} {
		build := exec.Command("go", "build", "-race", "-o", filepath.Join(testBinaries, command), "./cmd/"+command)
		build.Dir = root
		output, err := build.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "build %s: %v\n%s", command, err, output)
			os.Exit(1)
		}
	}
	code := m.Run()
	if err := os.RemoveAll(testBinaries); err != nil {
		fmt.Fprintf(os.Stderr, "remove semantic gate binaries: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func TestProjectRealityLifecycle(t *testing.T) {
	root := t.TempDir()
	repository := newRepository(t, filepath.Join(root, "repository"), "trunk")
	harness := newHarness(t, shortDataDir(t))
	daemon := harness.start(t)

	// 1. Open a clean repository without changing its checkout or Git metadata.
	beforeOpen := snapshotRepository(t, repository)
	opened := harness.open(t, repository, "trunk")
	afterOpen := snapshotRepository(t, repository)
	assertEqual(t, "repository after project open", beforeOpen, afterOpen)
	projectID := opened.Project.Project.ID
	cleanManifest := opened.Project.CurrentReality.ManifestDigest

	// 2. Resolve a real linked worktree to the same stable Project identity.
	linked := filepath.Join(root, "linked")
	runGit(t, repository, "worktree", "add", "--detach", linked, "trunk")
	beforeLinkedOpen := snapshotRepository(t, repository)
	linkedResult := harness.open(t, linked, "")
	if linkedResult.Project.Project.ID != projectID {
		t.Fatalf("linked worktree Project ID = %s, want %s", linkedResult.Project.Project.ID, projectID)
	}
	assertEqual(t, "repository after linked worktree open", beforeLinkedOpen, snapshotRepository(t, repository))

	// 3. Prove dirty sentinel bytes never enter committed Reality artifacts.
	trackedSentinel := "NEMETON_TRACKED_DIRTY_SENTINEL"
	untrackedSentinel := "NEMETON_UNTRACKED_DIRTY_SENTINEL"
	writeFile(t, filepath.Join(repository, "README.md"), []byte("# changed\n"+trackedSentinel+"\n"))
	writeFile(t, filepath.Join(repository, "untracked.txt"), []byte(untrackedSentinel+"\n"))
	beforeDirtyOpen := snapshotRepository(t, repository)
	dirtyResult := harness.open(t, repository, "")
	if !dirtyResult.Project.CurrentReality.DirtyObserved {
		t.Fatal("dirty checkout was not observed")
	}
	if dirtyResult.Project.CurrentReality.ManifestDigest != cleanManifest {
		t.Fatalf("dirty manifest digest = %s, want clean digest %s", dirtyResult.Project.CurrentReality.ManifestDigest, cleanManifest)
	}
	assertEqual(t, "repository after dirty open", beforeDirtyOpen, snapshotRepository(t, repository))
	assertArtifactsExclude(t, filepath.Join(harness.dataDir, "artifacts"), trackedSentinel, untrackedSentinel)

	// 4. Explicitly relink a moved repository without changing Project identity.
	runGit(t, repository, "worktree", "remove", linked)
	runGit(t, repository, "branch", "-m", "trunk", "renamed")
	assertProblemCode(t, harness, "integration_branch_not_found", "project", "open", repository, "--data-dir", harness.dataDir, "--json")
	movedRepository := filepath.Join(root, "repository-moved")
	if err := os.Rename(repository, movedRepository); err != nil {
		t.Fatalf("move repository: %v", err)
	}
	relinked := harness.relink(t, projectID, movedRepository, "renamed")
	if relinked.Project.Project.ID != projectID {
		t.Fatalf("relinked Project ID = %s, want %s", relinked.Project.Project.ID, projectID)
	}
	stableDigest := relinked.Project.ResultDigest

	// 5. Reject a second daemon and preserve state across a real restart.
	second := exec.Command(filepath.Join(testBinaries, "nemetond"), "--data-dir", harness.dataDir)
	secondOutput, secondErr := second.CombinedOutput()
	if secondErr == nil || !strings.Contains(string(secondOutput), "owns data directory lock") {
		t.Fatalf("second daemon result = %v, output = %s", secondErr, secondOutput)
	}
	daemon.stop(t)
	daemon = harness.start(t)
	inspected := harness.inspect(t, projectID)
	if inspected.Project.ResultDigest != stableDigest {
		t.Fatalf("restart digest = %s, want %s", inspected.Project.ResultDigest, stableDigest)
	}

	// 6. Delete derived projections and require maintenance before replay.
	daemon.stop(t)
	deleteProjectProjections(t, harness.databasePath(), projectID)
	daemon = harness.start(t)
	health := harness.health(t)
	if health.Status != "maintenance" || !reflect.DeepEqual(health.MaintenanceProjects, []string{projectID}) {
		t.Fatalf("maintenance health = %#v", health)
	}
	if _, _, exit := harness.command("project", "inspect", "--data-dir", harness.dataDir, "--json", projectID); exit == 0 {
		t.Fatal("inspect succeeded while daemon required replay")
	}
	beforeReplayRepository := snapshotRepository(t, movedRepository)
	beforeReplayArtifacts := snapshotFiles(t, filepath.Join(harness.dataDir, "artifacts"))
	replayed := harness.replay(t, projectID)
	if replayed.Project.ResultDigest != stableDigest {
		t.Fatalf("replay digest = %s, want %s", replayed.Project.ResultDigest, stableDigest)
	}
	if harness.health(t).Status != "ready" {
		t.Fatal("daemon did not return to ready after replay")
	}
	assertEqual(t, "repository after replay", beforeReplayRepository, snapshotRepository(t, movedRepository))
	assertEqual(t, "artifacts after replay", beforeReplayArtifacts, snapshotFiles(t, filepath.Join(harness.dataDir, "artifacts")))
	daemon.stop(t)
	assertEventHistoryImmutable(t, harness.databasePath())

	// 7. Corrupting a referenced artifact must prevent daemon readiness.
	corruptFirstArtifact(t, filepath.Join(harness.dataDir, "artifacts"))
	harness.startExpectFailure(t, "artifact")
}

func TestAtomicEventProjectionTransaction(t *testing.T) {
	root := t.TempDir()
	repository := newRepository(t, filepath.Join(root, "repository"), "trunk")
	harness := newHarness(t, shortDataDir(t))
	daemon := harness.start(t)
	opened := harness.open(t, repository, "trunk")
	projectID := opened.Project.Project.ID
	daemon.stop(t)

	before := databaseCounts(t, harness.databasePath(), projectID)
	database := openDatabase(t, harness.databasePath())
	if _, err := database.Exec(`
        CREATE TRIGGER force_reality_projection_failure
        BEFORE INSERT ON reality_revisions
        BEGIN
            SELECT RAISE(ABORT, 'forced semantic gate failure');
        END`); err != nil {
		database.Close()
		t.Fatalf("create real SQLite failure trigger: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close fault-injected database: %v", err)
	}

	daemon = harness.start(t)
	_, stderr, exit := harness.command("project", "open", "--data-dir", harness.dataDir, "--json", repository)
	if exit == 0 || !strings.Contains(stderr, "forced semantic gate failure") {
		t.Fatalf("faulted open exit = %d, stderr = %s", exit, stderr)
	}
	daemon.stop(t)
	after := databaseCounts(t, harness.databasePath(), projectID)
	assertEqual(t, "atomic event/projection rollback", before, after)
}

func TestRepositoryBoundaryFailures(t *testing.T) {
	root := t.TempDir()
	harness := newHarness(t, shortDataDir(t))
	daemon := harness.start(t)
	defer daemon.stop(t)

	unique := newRepository(t, filepath.Join(root, "unique"), "trunk")
	if selected := harness.open(t, unique, "").Project.Binding.IntegrationBranch; selected != "trunk" {
		t.Fatalf("unique local branch selection = %s, want trunk", selected)
	}

	remoteSource := newRepository(t, filepath.Join(root, "remote-source"), "trunk")
	bareRemote := filepath.Join(root, "remote.git")
	run(t, "git", "clone", "--bare", remoteSource, bareRemote)
	remoteClone := filepath.Join(root, "remote-clone")
	run(t, "git", "clone", bareRemote, remoteClone)
	if selected := harness.open(t, remoteClone, "").Project.Binding.IntegrationBranch; selected != "trunk" {
		t.Fatalf("remote HEAD branch selection = %s, want trunk", selected)
	}

	ambiguous := newRepository(t, filepath.Join(root, "ambiguous"), "trunk")
	runGit(t, ambiguous, "branch", "release")
	assertProblemCode(t, harness, "ambiguous_integration_branch", "project", "open", "--data-dir", harness.dataDir, "--json", ambiguous)

	nonGit := filepath.Join(root, "plain")
	if err := os.Mkdir(nonGit, 0o700); err != nil {
		t.Fatalf("create non-Git directory: %v", err)
	}
	assertProblemCode(t, harness, "not_git_repository", "project", "open", "--data-dir", harness.dataDir, "--json", nonGit)

	bare := filepath.Join(root, "bare.git")
	run(t, "git", "init", "--bare", bare)
	assertProblemCode(t, harness, "unsupported_bare_repository", "project", "open", "--data-dir", harness.dataDir, "--json", bare)

	broken := filepath.Join(root, "broken")
	if err := os.Mkdir(broken, 0o700); err != nil {
		t.Fatalf("create broken worktree: %v", err)
	}
	writeFile(t, filepath.Join(broken, ".git"), []byte("gitdir: /definitely/missing/nemeton-common-dir\n"))
	assertProblemCode(t, harness, "broken_git_common_dir", "project", "open", "--data-dir", harness.dataDir, "--json", broken)
}

func TestUnknownEventVersionPreventsStartup(t *testing.T) {
	root := t.TempDir()
	repository := newRepository(t, filepath.Join(root, "repository"), "trunk")
	harness := newHarness(t, shortDataDir(t))
	daemon := harness.start(t)
	opened := harness.open(t, repository, "trunk")
	daemon.stop(t)

	database := openDatabase(t, harness.databasePath())
	projectID := opened.Project.Project.ID
	sequence := opened.Project.StreamVersion + 1
	eventID, err := event.NewID()
	if err != nil {
		t.Fatalf("generate corrupt event ID: %v", err)
	}
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := database.Exec(`
        INSERT INTO domain_events(
            event_id, project_id, sequence, aggregate_type, aggregate_id,
            event_type, schema_version, payload_json, actor, causation_id,
            correlation_id, occurred_at, recorded_at
        ) VALUES (?, ?, ?, 'project', ?, 'FutureEvent.v999', 999, ?,
                  'semantic-gate', ?, ?, ?, ?)`,
		eventID, projectID, sequence, projectID, []byte("{}"), eventID, eventID, stamp, stamp); err != nil {
		database.Close()
		t.Fatalf("insert future event: %v", err)
	}
	if _, err := database.Exec(`UPDATE project_streams SET current_sequence = ? WHERE project_id = ?`, sequence, projectID); err != nil {
		database.Close()
		t.Fatalf("advance corrupt stream: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close future-event database: %v", err)
	}
	harness.startExpectFailure(t, "unsupported")
}

func TestFutureSchemaPreventsStartup(t *testing.T) {
	harness := newHarness(t, shortDataDir(t))
	daemon := harness.start(t)
	daemon.stop(t)

	database := openDatabase(t, harness.databasePath())
	if _, err := database.Exec(`INSERT INTO schema_migrations(version, name, applied_at) VALUES (999, 'future.sql', 'future')`); err != nil {
		database.Close()
		t.Fatalf("insert future schema version: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close future-schema database: %v", err)
	}
	harness.startExpectFailure(t, "newer than supported")
}

type harness struct {
	dataDir string
}

type daemonProcess struct {
	command *exec.Cmd
	stderr  *bytes.Buffer
	done    chan error
}

func newHarness(t *testing.T, dataDir string) *harness {
	t.Helper()
	return &harness{dataDir: dataDir}
}

func shortDataDir(t *testing.T) string {
	// Keep the real Unix Socket under the shortest shared path on supported hosts.
	root, err := os.MkdirTemp("/tmp", "nemeton-gate-")
	if err != nil {
		t.Fatalf("create short semantic gate directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove short semantic gate directory: %v", err)
		}
	})
	return filepath.Join(root, "data")
}

func (h *harness) start(t *testing.T) *daemonProcess {
	t.Helper()
	var stderr bytes.Buffer
	command := exec.Command(filepath.Join(testBinaries, "nemetond"), "--data-dir", h.dataDir)
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatalf("start nemetond: %v", err)
	}
	process := &daemonProcess{command: command, stderr: &stderr, done: make(chan error, 1)}
	go func() { process.done <- command.Wait() }()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, exit := h.command("daemon", "status", "--data-dir", h.dataDir, "--json"); exit == 0 {
			return process
		}
		select {
		case err := <-process.done:
			t.Fatalf("nemetond exited before ready: %v\n%s", err, stderr.String())
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
	process.command.Process.Kill()
	t.Fatalf("nemetond did not expose health within 10s\n%s", stderr.String())
	return nil
}

func (h *harness) startExpectFailure(t *testing.T, detail string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, filepath.Join(testBinaries, "nemetond"), "--data-dir", h.dataDir)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("nemetond unexpectedly started; wanted failure containing %q", detail)
	}
	if ctx.Err() != nil {
		t.Fatalf("nemetond did not fail within 10s: %v", ctx.Err())
	}
	if !strings.Contains(strings.ToLower(string(output)), strings.ToLower(detail)) {
		t.Fatalf("nemetond failure = %s, want detail %q", output, detail)
	}
}

func (p *daemonProcess) stop(t *testing.T) {
	t.Helper()
	select {
	case err := <-p.done:
		if err != nil {
			t.Fatalf("nemetond exited before stop: %v\n%s", err, p.stderr.String())
		}
		return
	default:
	}
	if err := p.command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("signal nemetond: %v", err)
	}
	select {
	case err := <-p.done:
		if err != nil {
			t.Fatalf("stop nemetond: %v\n%s", err, p.stderr.String())
		}
	case <-time.After(10 * time.Second):
		p.command.Process.Kill()
		t.Fatalf("nemetond did not stop within 10s\n%s", p.stderr.String())
	}
}

func (h *harness) command(args ...string) (string, string, int) {
	command := exec.Command(filepath.Join(testBinaries, "nemeton"), args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		return stdout.String(), stderr.String() + err.Error(), -1
	}
	return stdout.String(), stderr.String(), exitError.ExitCode()
}

func (h *harness) health(t *testing.T) api.Health {
	t.Helper()
	stdout, stderr, exit := h.command("daemon", "status", "--data-dir", h.dataDir, "--json")
	if exit != 0 {
		t.Fatalf("daemon status exit = %d, stderr = %s", exit, stderr)
	}
	var health api.Health
	decodeJSON(t, stdout, &health)
	return health
}

func (h *harness) open(t *testing.T, repository, branch string) api.ProjectResponse {
	t.Helper()
	args := []string{"project", "open", repository, "--data-dir", h.dataDir, "--json"}
	if branch != "" {
		args = append(args, "--integration-branch", branch)
	}
	return h.projectCommand(t, args...)
}

func (h *harness) inspect(t *testing.T, projectID string) api.ProjectResponse {
	t.Helper()
	return h.projectCommand(t, "project", "inspect", projectID, "--data-dir", h.dataDir, "--json")
}

func (h *harness) relink(t *testing.T, projectID, repository, branch string) api.ProjectResponse {
	t.Helper()
	return h.projectCommand(t, "project", "relink", projectID, repository, "--data-dir", h.dataDir, "--integration-branch", branch, "--json")
}

func (h *harness) replay(t *testing.T, projectID string) api.ProjectResponse {
	t.Helper()
	return h.projectCommand(t, "project", "replay", projectID, "--data-dir", h.dataDir, "--json")
}

func (h *harness) projectCommand(t *testing.T, args ...string) api.ProjectResponse {
	t.Helper()
	stdout, stderr, exit := h.command(args...)
	if exit != 0 {
		t.Fatalf("nemeton %v exit = %d, stderr = %s", args, exit, stderr)
	}
	var response api.ProjectResponse
	decodeJSON(t, stdout, &response)
	return response
}

func (h *harness) databasePath() string {
	return filepath.Join(h.dataDir, "nemeton.db")
}

func newRepository(t *testing.T, path, branch string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("create repository directory: %v", err)
	}
	runGit(t, path, "init", "-b", branch)
	runGit(t, path, "config", "user.name", "Nemeton Semantic Gate")
	runGit(t, path, "config", "user.email", "nemeton@example.invalid")
	writeFile(t, filepath.Join(path, "README.md"), []byte("# Semantic Gate\n"))
	runGit(t, path, "add", "README.md")
	runGit(t, path, "commit", "-m", "initial")
	return path
}

func runGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-C", directory}, args...)
	command := exec.Command("git", commandArgs...)
	command.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", commandArgs, err, output)
	}
	return strings.TrimSpace(string(output))
}

func run(t *testing.T, executable string, args ...string) string {
	t.Helper()
	output, err := exec.Command(executable, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", executable, args, err, output)
	}
	return strings.TrimSpace(string(output))
}

type repositorySnapshot struct {
	Status     string
	Refs       string
	IndexHash  string
	ConfigHash string
	Files      map[string]string
}

func snapshotRepository(t *testing.T, repository string) repositorySnapshot {
	t.Helper()
	status := runGit(t, repository, "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "status", "--porcelain=v2", "--untracked-files=all")
	refs := runGit(t, repository, "for-each-ref", "--format=%(refname) %(objectname)")
	indexPath := runGit(t, repository, "rev-parse", "--path-format=absolute", "--git-path", "index")
	configPath := runGit(t, repository, "rev-parse", "--path-format=absolute", "--git-path", "config")
	return repositorySnapshot{
		Status:     status,
		Refs:       refs,
		IndexHash:  hashFile(t, indexPath),
		ConfigHash: hashFile(t, configPath),
		Files:      snapshotWorkingFiles(t, repository),
	}
}

func snapshotWorkingFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == ".git" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if relative == "." || entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		result[filepath.ToSlash(relative)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot working files: %v", err)
	}
	return result
}

func snapshotFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = hashFile(t, path)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot files in %s: %v", root, err)
	}
	return result
}

func hashFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s for digest: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func assertArtifactsExclude(t *testing.T, root string, sentinels ...string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, sentinel := range sentinels {
			if bytes.Contains(data, []byte(sentinel)) {
				return fmt.Errorf("artifact %s contains dirty sentinel %q", path, sentinel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func deleteProjectProjections(t *testing.T, path, projectID string) {
	t.Helper()
	database := openDatabase(t, path)
	defer database.Close()
	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin projection deletion: %v", err)
	}
	for _, statement := range []string{
		`DELETE FROM projection_states WHERE project_id = ?`,
		`DELETE FROM reality_revisions WHERE project_id = ?`,
		`DELETE FROM repository_bindings WHERE project_id = ?`,
		`DELETE FROM projects WHERE id = ?`,
	} {
		if _, err := transaction.Exec(statement, projectID); err != nil {
			transaction.Rollback()
			t.Fatalf("delete Project projection: %v", err)
		}
	}
	if err := transaction.Commit(); err != nil {
		t.Fatalf("commit projection deletion: %v", err)
	}
}

type counts struct {
	Events     int
	Stream     int64
	Projects   int
	Bindings   int
	Realities  int
	Projection string
}

func databaseCounts(t *testing.T, path, projectID string) counts {
	t.Helper()
	database := openDatabase(t, path)
	defer database.Close()
	var result counts
	queries := []struct {
		query  string
		target any
	}{
		{`SELECT COUNT(*) FROM domain_events WHERE project_id = ?`, &result.Events},
		{`SELECT current_sequence FROM project_streams WHERE project_id = ?`, &result.Stream},
		{`SELECT COUNT(*) FROM projects WHERE id = ?`, &result.Projects},
		{`SELECT COUNT(*) FROM repository_bindings WHERE project_id = ?`, &result.Bindings},
		{`SELECT COUNT(*) FROM reality_revisions WHERE project_id = ?`, &result.Realities},
		{`SELECT result_digest FROM projection_states WHERE project_id = ? AND projection_name = 'project'`, &result.Projection},
	}
	for _, item := range queries {
		if err := database.QueryRow(item.query, projectID).Scan(item.target); err != nil {
			t.Fatalf("query atomic state with %q: %v", item.query, err)
		}
	}
	return result
}

func openDatabase(t *testing.T, path string) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", "file:"+path+"?_txlock=immediate")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := database.Ping(); err != nil {
		database.Close()
		t.Fatalf("connect test database: %v", err)
	}
	return database
}

func corruptFirstArtifact(t *testing.T, root string) {
	t.Helper()
	var target string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && target == "" {
			target = path
		}
		return nil
	})
	if err != nil {
		t.Fatalf("find artifact to corrupt: %v", err)
	}
	if target == "" {
		t.Fatal("no artifact available to corrupt")
	}
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("open artifact for corruption: %v", err)
	}
	if _, err := file.Write([]byte("corrupt")); err != nil {
		file.Close()
		t.Fatalf("corrupt artifact: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close corrupted artifact: %v", err)
	}
}

func assertEventHistoryImmutable(t *testing.T, path string) {
	t.Helper()
	database := openDatabase(t, path)
	defer database.Close()
	for _, statement := range []string{
		`UPDATE domain_events SET actor = 'changed'`,
		`DELETE FROM domain_events`,
		`UPDATE event_artifacts SET relation = 'changed'`,
		`DELETE FROM event_artifacts`,
	} {
		if _, err := database.Exec(statement); err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Fatalf("append-only statement %q returned %v", statement, err)
		}
	}
}

func assertProblemCode(t *testing.T, harness *harness, expected string, args ...string) {
	t.Helper()
	_, stderr, exit := harness.command(args...)
	if exit == 0 {
		t.Fatalf("nemeton %v unexpectedly succeeded", args)
	}
	var problem api.Problem
	decodeJSON(t, stderr, &problem)
	if problem.Code != expected {
		t.Fatalf("problem code = %s, want %s; detail = %s", problem.Code, expected, problem.Detail)
	}
}

func decodeJSON(t *testing.T, data string, target any) {
	t.Helper()
	if err := json.Unmarshal([]byte(data), target); err != nil {
		t.Fatalf("decode JSON %q: %v", data, err)
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertEqual(t *testing.T, label string, want, got any) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("%s changed\nwant: %#v\n got: %#v", label, want, got)
	}
}

func repositoryRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve semantic gate source path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..")), nil
}
