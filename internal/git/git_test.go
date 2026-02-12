package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial commit"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "cmd %v failed: %s", args, out)
	}

	return dir
}

func TestOps_CurrentBranch(t *testing.T) {
	t.Parallel()
	repo := initTestRepo(t)
	ops := NewOps(repo)

	branch, err := ops.CurrentBranch(context.Background())
	require.NoError(t, err)
	assert.Contains(t, []string{"main", "master"}, branch)
}

func TestOps_IsClean_WhenClean(t *testing.T) {
	t.Parallel()
	repo := initTestRepo(t)
	ops := NewOps(repo)

	clean, err := ops.IsClean(context.Background())
	require.NoError(t, err)
	assert.True(t, clean)
}

func TestOps_IsClean_WhenDirty(t *testing.T) {
	t.Parallel()
	repo := initTestRepo(t)
	ops := NewOps(repo)

	require.NoError(t, os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("change"), 0644))

	clean, err := ops.IsClean(context.Background())
	require.NoError(t, err)
	assert.False(t, clean)
}

func TestOps_CreateBranchAndCheckout(t *testing.T) {
	t.Parallel()
	repo := initTestRepo(t)
	ops := NewOps(repo)
	ctx := context.Background()

	defaultBranch, err := ops.CurrentBranch(ctx)
	require.NoError(t, err)

	require.NoError(t, ops.CreateBranch(ctx, "herald/test-branch"))

	branch, err := ops.CurrentBranch(ctx)
	require.NoError(t, err)
	assert.Equal(t, "herald/test-branch", branch)

	// Switch back to original branch
	require.NoError(t, ops.Checkout(ctx, defaultBranch))

	branch, err = ops.CurrentBranch(ctx)
	require.NoError(t, err)
	assert.Equal(t, defaultBranch, branch)
}

func TestOps_Diff_ShowsChanges(t *testing.T) {
	t.Parallel()
	repo := initTestRepo(t)
	ops := NewOps(repo)
	ctx := context.Background()

	defaultBranch, err := ops.CurrentBranch(ctx)
	require.NoError(t, err)

	// Create a branch, add a file, commit
	require.NoError(t, ops.CreateBranch(ctx, "feature"))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "new.txt"), []byte("content"), 0644))

	cmd := exec.Command("git", "add", "new.txt")
	cmd.Dir = repo
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "add file")
	cmd.Dir = repo
	require.NoError(t, cmd.Run())

	// Get diff against default branch
	diff, err := ops.Diff(ctx, defaultBranch, "feature")
	require.NoError(t, err)
	assert.Contains(t, diff, "new.txt")
}

func TestOps_IsGitRepo(t *testing.T) {
	t.Parallel()
	repo := initTestRepo(t)
	ops := NewOps(repo)

	assert.True(t, ops.IsGitRepo(context.Background()))

	notRepo := NewOps(t.TempDir())
	assert.False(t, notRepo.IsGitRepo(context.Background()))
}

func TestOps_Log(t *testing.T) {
	t.Parallel()
	repo := initTestRepo(t)
	ops := NewOps(repo)

	log, err := ops.Log(context.Background(), 5)
	require.NoError(t, err)
	assert.Contains(t, log, "initial commit")
}

func TestOps_HasCommits_WhenRepo_HasCommits(t *testing.T) {
	t.Parallel()
	repo := initTestRepo(t)
	ops := NewOps(repo)

	assert.True(t, ops.HasCommits(context.Background()))
}

func TestOps_HasCommits_WhenRepo_HasNoCommits(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	require.NoError(t, cmd.Run())

	ops := NewOps(dir)
	assert.False(t, ops.HasCommits(context.Background()))
}

func TestOps_Diff_WhenUncommittedChanges(t *testing.T) {
	t.Parallel()
	repo := initTestRepo(t)
	ops := NewOps(repo)
	ctx := context.Background()

	// Create and commit a file
	require.NoError(t, os.WriteFile(filepath.Join(repo, "file.txt"), []byte("initial"), 0644))
	cmd := exec.Command("git", "add", "file.txt")
	cmd.Dir = repo
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "commit", "-m", "add file")
	cmd.Dir = repo
	require.NoError(t, cmd.Run())

	// Modify the file (uncommitted)
	require.NoError(t, os.WriteFile(filepath.Join(repo, "file.txt"), []byte("modified"), 0644))

	diff, err := ops.Diff(ctx, "HEAD", "")
	require.NoError(t, err)
	assert.Contains(t, diff, "file.txt")
	assert.Contains(t, diff, "modified")
}

func TestOps_DiffStat(t *testing.T) {
	t.Parallel()
	repo := initTestRepo(t)
	ops := NewOps(repo)
	ctx := context.Background()

	defaultBranch, err := ops.CurrentBranch(ctx)
	require.NoError(t, err)

	require.NoError(t, ops.CreateBranch(ctx, "stats-test"))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "stat.txt"), []byte("data"), 0644))
	cmd := exec.Command("git", "add", "stat.txt")
	cmd.Dir = repo
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "commit", "-m", "add stat file")
	cmd.Dir = repo
	require.NoError(t, cmd.Run())

	stat, err := ops.DiffStat(ctx, defaultBranch, "stats-test")
	require.NoError(t, err)
	assert.Contains(t, stat, "stat.txt")
}

func TestOps_CreateBranch_WhenExistingBranches(t *testing.T) {
	t.Parallel()
	repo := initTestRepo(t)
	ops := NewOps(repo)
	ctx := context.Background()

	defaultBranch, err := ops.CurrentBranch(ctx)
	require.NoError(t, err)

	require.NoError(t, ops.CreateBranch(ctx, "branch-1"))
	require.NoError(t, ops.Checkout(ctx, defaultBranch))
	require.NoError(t, ops.CreateBranch(ctx, "branch-2"))

	branch, err := ops.CurrentBranch(ctx)
	require.NoError(t, err)
	assert.Equal(t, "branch-2", branch)
}

func TestOps_CreateBranch_WhenAlreadyExists_ReturnsError(t *testing.T) {
	t.Parallel()
	repo := initTestRepo(t)
	ops := NewOps(repo)
	ctx := context.Background()

	require.NoError(t, ops.CreateBranch(ctx, "existing"))
	require.NoError(t, ops.Checkout(ctx, "main"))

	err := ops.CreateBranch(ctx, "existing")
	assert.Error(t, err)
}

func TestOps_StashAndPop(t *testing.T) {
	t.Parallel()
	repo := initTestRepo(t)
	ops := NewOps(repo)
	ctx := context.Background()

	// Create a dirty file
	require.NoError(t, os.WriteFile(filepath.Join(repo, "stash-me.txt"), []byte("data"), 0644))

	cmd := exec.Command("git", "add", "stash-me.txt")
	cmd.Dir = repo
	require.NoError(t, cmd.Run())

	require.NoError(t, ops.Stash(ctx))

	clean, err := ops.IsClean(ctx)
	require.NoError(t, err)
	assert.True(t, clean, "should be clean after stash")

	require.NoError(t, ops.StashPop(ctx))

	clean, err = ops.IsClean(ctx)
	require.NoError(t, err)
	assert.False(t, clean, "should be dirty after pop")
}
