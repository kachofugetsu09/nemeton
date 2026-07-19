package gitrepo

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kachofugetsu09/nemeton/internal/event"
)

const MinimumGitVersion = "2.41.0"

var versionPattern = regexp.MustCompile(`^git version ([0-9]+)\.([0-9]+)\.([0-9]+)`)

type Repository struct {
	DisplayPath         string
	CanonicalRoot       string
	GitCommonDir        string
	RemoteIdentities    []event.RemoteIdentity
	DiscoveredWorktrees []string
	LocalBranches       []string
	RemoteHeads         []RemoteHead
}

type RemoteHead struct {
	Remote string `json:"remote"`
	Branch string `json:"branch"`
}

type Capture struct {
	IntegrationBranch string
	CommitOID         string
	TreeOID           string
	DirtyObserved     bool
	Manifest          Manifest
}

type Manifest struct {
	Schema            string      `json:"schema"`
	IntegrationBranch string      `json:"integration_branch"`
	CommitOID         string      `json:"commit_oid"`
	TreeOID           string      `json:"tree_oid"`
	Entries           []TreeEntry `json:"entries"`
	Inputs            []Input     `json:"inputs"`
	Checks            []Check     `json:"checks"`
}

type TreeEntry struct {
	Path         string `json:"path"`
	PathEncoding string `json:"path_encoding"`
	Mode         string `json:"mode"`
	ObjectType   string `json:"object_type"`
	ObjectID     string `json:"object_id"`
}

type Input struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

type Check struct {
	Kind          string   `json:"kind"`
	Status        string   `json:"status"`
	Command       string   `json:"command,omitempty"`
	EvidencePaths []string `json:"evidence_paths,omitempty"`
}

type Error struct {
	Code   string
	Detail string
	Err    error
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return e.Detail
	}
	return e.Err.Error()
}

func (e *Error) Unwrap() error {
	return e.Err
}

func CheckVersion(ctx context.Context) error {
	result, err := run(ctx, "", nil, "--version")
	if err != nil {
		return err
	}
	match := versionPattern.FindStringSubmatch(strings.TrimSpace(string(result)))
	if match == nil {
		return fmt.Errorf("parse Git version output: %q", strings.TrimSpace(string(result)))
	}
	var actual [3]int
	for index, value := range match[1:] {
		part, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse Git version component %q: %w", value, err)
		}
		actual[index] = part
	}
	minimum := [3]int{2, 41, 0}
	if lessVersion(actual, minimum) {
		return fmt.Errorf("Git %s or newer is required; found %d.%d.%d", MinimumGitVersion, actual[0], actual[1], actual[2])
	}
	return nil
}

func Discover(ctx context.Context, path string) (Repository, error) {
	// 1. Validate and canonicalize the user-selected worktree.
	if !filepath.IsAbs(path) {
		return Repository{}, &Error{Code: "invalid_argument", Detail: "repository path must be absolute"}
	}
	canonicalInput, err := filepath.EvalSymlinks(path)
	if err != nil {
		return Repository{}, &Error{Code: "not_git_repository", Detail: fmt.Sprintf("resolve repository path %s: %v", path, err), Err: err}
	}
	bare, err := gitText(ctx, canonicalInput, "rev-parse", "--is-bare-repository")
	if err != nil {
		if brokenGitPointer(canonicalInput) {
			return Repository{}, &Error{Code: "broken_git_common_dir", Detail: fmt.Sprintf("Git metadata points to a missing common-dir: %s", path), Err: err}
		}
		return Repository{}, &Error{Code: "not_git_repository", Detail: fmt.Sprintf("not a Git repository: %s", path), Err: err}
	}
	if bare == "true" {
		return Repository{}, &Error{Code: "unsupported_bare_repository", Detail: fmt.Sprintf("bare repositories are not supported: %s", path)}
	}
	inside, err := gitText(ctx, canonicalInput, "rev-parse", "--is-inside-work-tree")
	if err != nil || inside != "true" {
		return Repository{}, &Error{Code: "not_git_repository", Detail: fmt.Sprintf("not a Git worktree: %s", path), Err: err}
	}

	// 2. Resolve the shared repository identity.
	topLevel, err := gitText(ctx, canonicalInput, "rev-parse", "--path-format=absolute", "--show-toplevel")
	if err != nil {
		return Repository{}, err
	}
	commonDir, err := gitText(ctx, canonicalInput, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return Repository{}, err
	}
	canonicalRoot, err := filepath.EvalSymlinks(topLevel)
	if err != nil {
		return Repository{}, &Error{Code: "broken_git_common_dir", Detail: fmt.Sprintf("resolve Git top-level %s: %v", topLevel, err), Err: err}
	}
	canonicalCommon, err := filepath.EvalSymlinks(commonDir)
	if err != nil {
		return Repository{}, &Error{Code: "broken_git_common_dir", Detail: fmt.Sprintf("resolve Git common-dir %s: %v", commonDir, err), Err: err}
	}
	info, err := os.Stat(canonicalCommon)
	if err != nil || !info.IsDir() {
		return Repository{}, &Error{Code: "broken_git_common_dir", Detail: fmt.Sprintf("Git common-dir is not a directory: %s", canonicalCommon), Err: err}
	}

	// 3. Read local-only binding evidence.
	remotes, err := discoverRemotes(ctx, canonicalRoot)
	if err != nil {
		return Repository{}, err
	}
	worktrees, err := discoverWorktrees(ctx, canonicalRoot)
	if err != nil {
		return Repository{}, err
	}
	branches, err := localBranches(ctx, canonicalRoot)
	if err != nil {
		return Repository{}, err
	}
	remoteHeads, err := discoverRemoteHeads(ctx, canonicalRoot, branches)
	if err != nil {
		return Repository{}, err
	}
	return Repository{
		DisplayPath:         canonicalRoot,
		CanonicalRoot:       canonicalRoot,
		GitCommonDir:        canonicalCommon,
		RemoteIdentities:    remotes,
		DiscoveredWorktrees: worktrees,
		LocalBranches:       branches,
		RemoteHeads:         remoteHeads,
	}, nil
}

func (r Repository) SelectIntegrationBranch(ctx context.Context, requested string) (string, error) {
	if requested != "" {
		if strings.HasPrefix(requested, "-") {
			return "", &Error{Code: "invalid_argument", Detail: fmt.Sprintf("invalid integration branch: %s", requested)}
		}
		if _, err := gitText(ctx, r.CanonicalRoot, "check-ref-format", "--branch", requested); err != nil {
			return "", &Error{Code: "invalid_argument", Detail: fmt.Sprintf("invalid integration branch %q", requested), Err: err}
		}
		if err := verifyLocalBranch(ctx, r.CanonicalRoot, requested); err != nil {
			return "", &Error{Code: "integration_branch_not_found", Detail: fmt.Sprintf("local integration branch not found: %s", requested), Err: err}
		}
		return requested, nil
	}
	if len(r.RemoteHeads) == 1 {
		return r.RemoteHeads[0].Branch, nil
	}
	if len(r.RemoteHeads) == 0 && len(r.LocalBranches) == 1 {
		return r.LocalBranches[0], nil
	}
	return "", &Error{
		Code:   "ambiguous_integration_branch",
		Detail: fmt.Sprintf("integration branch is ambiguous; local branches=%v remote HEADs=%v; specify --integration-branch", r.LocalBranches, r.RemoteHeads),
	}
}

func (r Repository) Capture(ctx context.Context, branch string) (Capture, []byte, error) {
	// 1. Resolve committed integration objects before observing the checkout.
	if err := verifyLocalBranch(ctx, r.CanonicalRoot, branch); err != nil {
		return Capture{}, nil, &Error{Code: "integration_branch_not_found", Detail: fmt.Sprintf("bound integration branch not found: %s", branch), Err: err}
	}
	commit, err := gitText(ctx, r.CanonicalRoot, "rev-parse", "--verify", "refs/heads/"+branch+"^{commit}")
	if err != nil {
		return Capture{}, nil, err
	}
	tree, err := gitText(ctx, r.CanonicalRoot, "rev-parse", "--verify", commit+"^{tree}")
	if err != nil {
		return Capture{}, nil, err
	}
	entries, err := treeEntries(ctx, r.CanonicalRoot, tree)
	if err != nil {
		return Capture{}, nil, err
	}

	// 2. Observe dirty state without allowing optional Git writes.
	status, err := run(ctx, r.CanonicalRoot, []string{"GIT_OPTIONAL_LOCKS=0"}, "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "status", "--porcelain=v2", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil {
		return Capture{}, nil, err
	}
	dirty := len(status) > 0
	confirmedCommit, err := gitText(ctx, r.CanonicalRoot, "rev-parse", "--verify", "refs/heads/"+branch+"^{commit}")
	if err != nil {
		return Capture{}, nil, err
	}
	if confirmedCommit != commit {
		return Capture{}, nil, &Error{Code: "git_observation_changed", Detail: fmt.Sprintf("integration branch %s changed during Reality capture", branch)}
	}

	// 3. Serialize only committed evidence into the content-addressed manifest.
	manifest := Manifest{
		Schema:            "nemeton.reality.v1",
		IntegrationBranch: branch,
		CommitOID:         commit,
		TreeOID:           tree,
		Entries:           entries,
		Inputs:            classifyInputs(entries),
		Checks:            discoverChecks(entries),
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return Capture{}, nil, fmt.Errorf("encode Reality manifest: %w", err)
	}
	return Capture{IntegrationBranch: branch, CommitOID: commit, TreeOID: tree, DirtyObserved: dirty, Manifest: manifest}, data, nil
}

func discoverRemotes(ctx context.Context, root string) ([]event.RemoteIdentity, error) {
	output, err := run(ctx, root, nil, "config", "--local", "--get-regexp", `^remote\..*\.url$`)
	if err != nil {
		var commandError *CommandError
		if errors.As(err, &commandError) && commandError.ExitCode == 1 {
			return []event.RemoteIdentity{}, nil
		}
		return nil, err
	}
	var identities []event.RemoteIdentity
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		key, rawURL, found := strings.Cut(line, " ")
		if !found {
			return nil, fmt.Errorf("parse Git remote config line: %q", line)
		}
		name := strings.TrimSuffix(strings.TrimPrefix(key, "remote."), ".url")
		identities = append(identities, event.RemoteIdentity{Name: name, URL: redactURL(rawURL)})
	}
	sort.Slice(identities, func(i, j int) bool {
		if identities[i].Name == identities[j].Name {
			return identities[i].URL < identities[j].URL
		}
		return identities[i].Name < identities[j].Name
	})
	return identities, nil
}

func discoverWorktrees(ctx context.Context, root string) ([]string, error) {
	output, err := run(ctx, root, nil, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, field := range bytes.Split(output, []byte{0}) {
		if bytes.HasPrefix(field, []byte("worktree ")) {
			path := string(bytes.TrimPrefix(field, []byte("worktree ")))
			canonical, err := filepath.EvalSymlinks(path)
			if err != nil {
				return nil, &Error{Code: "broken_git_common_dir", Detail: fmt.Sprintf("resolve discovered worktree %s: %v", path, err), Err: err}
			}
			paths = append(paths, canonical)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func localBranches(ctx context.Context, root string) ([]string, error) {
	output, err := run(ctx, root, nil, "for-each-ref", "--format=%(refname:strip=2)", "refs/heads")
	if err != nil {
		return nil, err
	}
	branches := nonemptyLines(output)
	sort.Strings(branches)
	return branches, nil
}

func discoverRemoteHeads(ctx context.Context, root string, branches []string) ([]RemoteHead, error) {
	output, err := run(ctx, root, nil, "for-each-ref", "--format=%(refname)%09%(symref)", "refs/remotes")
	if err != nil {
		return nil, err
	}
	local := make(map[string]bool, len(branches))
	for _, branch := range branches {
		local[branch] = true
	}
	var heads []RemoteHead
	for _, line := range nonemptyLines(output) {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 || !strings.HasSuffix(parts[0], "/HEAD") || parts[1] == "" {
			continue
		}
		remoteRef := strings.TrimPrefix(parts[0], "refs/remotes/")
		remote := strings.TrimSuffix(remoteRef, "/HEAD")
		targetPrefix := "refs/remotes/" + remote + "/"
		if !strings.HasPrefix(parts[1], targetPrefix) {
			continue
		}
		branch := strings.TrimPrefix(parts[1], targetPrefix)
		if local[branch] {
			heads = append(heads, RemoteHead{Remote: remote, Branch: branch})
		}
	}
	sort.Slice(heads, func(i, j int) bool {
		if heads[i].Remote == heads[j].Remote {
			return heads[i].Branch < heads[j].Branch
		}
		return heads[i].Remote < heads[j].Remote
	})
	return heads, nil
}

func treeEntries(ctx context.Context, root, tree string) ([]TreeEntry, error) {
	output, err := run(ctx, root, nil, "ls-tree", "-r", "-z", "--full-tree", tree)
	if err != nil {
		return nil, err
	}
	var entries []TreeEntry
	for _, record := range bytes.Split(output, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		metadata, path, found := bytes.Cut(record, []byte{'\t'})
		if !found {
			return nil, fmt.Errorf("parse Git tree record without path separator")
		}
		fields := bytes.Fields(metadata)
		if len(fields) != 3 {
			return nil, fmt.Errorf("parse Git tree metadata: %q", metadata)
		}
		encodedPath, encoding := encodePath(path)
		entries = append(entries, TreeEntry{Path: encodedPath, PathEncoding: encoding, Mode: string(fields[0]), ObjectType: string(fields[1]), ObjectID: string(fields[2])})
	}
	return entries, nil
}

func classifyInputs(entries []TreeEntry) []Input {
	var inputs []Input
	for _, entry := range entries {
		if entry.PathEncoding != "utf-8" {
			continue
		}
		path := filepath.ToSlash(entry.Path)
		lower := strings.ToLower(path)
		base := strings.ToLower(filepath.Base(path))
		kind := ""
		switch {
		case base == "agents.md" || base == "claude.md":
			kind = "agent_instructions"
		case strings.HasPrefix(base, "readme"):
			kind = "readme"
		case strings.HasPrefix(lower, "docs/"):
			kind = "documentation"
		case isBuildConfig(lower):
			kind = "build_config"
		case isCIConfig(lower):
			kind = "ci_config"
		}
		if kind != "" {
			inputs = append(inputs, Input{Kind: kind, Path: path})
		}
	}
	return inputs
}

func discoverChecks(entries []TreeEntry) []Check {
	checks := []Check{
		{Kind: "test", Status: "unknown"},
		{Kind: "static_analysis", Status: "unknown"},
		{Kind: "ci", Status: "unknown"},
		{Kind: "deployment", Status: "unknown"},
	}
	var ciEvidence []string
	for _, entry := range entries {
		if entry.PathEncoding != "utf-8" {
			continue
		}
		path := filepath.ToSlash(entry.Path)
		lower := strings.ToLower(path)
		if lower == "go.mod" {
			checks[0] = Check{Kind: "test", Status: "declared", Command: "go test ./...", EvidencePaths: []string{path}}
			checks[1] = Check{Kind: "static_analysis", Status: "declared", Command: "go vet ./...", EvidencePaths: []string{path}}
		}
		if isCIConfig(lower) {
			ciEvidence = append(ciEvidence, path)
		}
	}
	if len(ciEvidence) > 0 {
		sort.Strings(ciEvidence)
		checks[2] = Check{Kind: "ci", Status: "declared", EvidencePaths: ciEvidence}
	}
	return checks
}

func verifyLocalBranch(ctx context.Context, root, branch string) error {
	_, err := run(ctx, root, nil, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err
}

func gitText(ctx context.Context, root string, args ...string) (string, error) {
	output, err := run(ctx, root, nil, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

type CommandError struct {
	Args     []string
	ExitCode int
	Stderr   string
}

func (e *CommandError) Error() string {
	return fmt.Sprintf("git %s failed with exit code %d: %s", strings.Join(e.Args, " "), e.ExitCode, strings.TrimSpace(e.Stderr))
}

func run(ctx context.Context, root string, extraEnv []string, args ...string) ([]byte, error) {
	commandArgs := make([]string, 0, len(args)+2)
	if root != "" {
		commandArgs = append(commandArgs, "-C", root)
	}
	commandArgs = append(commandArgs, args...)
	command := exec.CommandContext(ctx, "git", commandArgs...)
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	command.Env = append(command.Env, extraEnv...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return nil, &CommandError{Args: commandArgs, ExitCode: exitError.ExitCode(), Stderr: stderr.String()}
		}
		return nil, fmt.Errorf("execute git %s: %w", strings.Join(commandArgs, " "), err)
	}
	return stdout.Bytes(), nil
}

func redactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.User == nil {
		return raw
	}
	if parsed.Scheme == "http" || parsed.Scheme == "https" {
		parsed.User = nil
	} else if _, hasPassword := parsed.User.Password(); hasPassword {
		parsed.User = url.User(parsed.User.Username())
	}
	return parsed.String()
}

func encodePath(path []byte) (string, string) {
	if utf8.Valid(path) {
		return string(path), "utf-8"
	}
	return base64.RawStdEncoding.EncodeToString(path), "base64"
}

func isBuildConfig(path string) bool {
	switch path {
	case "go.mod", "go.sum", "package.json", "pnpm-lock.yaml", "yarn.lock", "package-lock.json", "pyproject.toml", "poetry.lock", "cargo.toml", "cargo.lock", "pom.xml", "build.gradle", "build.gradle.kts", "makefile":
		return true
	default:
		return false
	}
}

func isCIConfig(path string) bool {
	return strings.HasPrefix(path, ".github/workflows/") || strings.HasPrefix(path, ".circleci/") || path == ".gitlab-ci.yml" || path == "jenkinsfile" || path == "azure-pipelines.yml"
}

func nonemptyLines(output []byte) []string {
	var result []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func lessVersion(actual, minimum [3]int) bool {
	for index := range actual {
		if actual[index] != minimum[index] {
			return actual[index] < minimum[index]
		}
	}
	return false
}

func brokenGitPointer(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, ".git"))
	if err != nil {
		return false
	}
	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(line, "gitdir: ") {
		return false
	}
	target := strings.TrimPrefix(line, "gitdir: ")
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	_, err = os.Stat(target)
	return os.IsNotExist(err)
}
