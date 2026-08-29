package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"mem/application/usecases"
	"mem/domain"
)

func CmdReview(deps *Deps, args []string) {
	if len(args) == 0 {
		fail("uso: mem review --diff [rango] | --commit <sha> | --file <ruta>\n" +
			"     mem review status [<review-id>] | history [--limit N] | show <review-id>")
	}
	var targetType domain.TargetType
	var revision, digest string
	var scope []string
	var err error
	switch args[0] {
	case "--diff":
		var diffRange string
		if len(args) > 1 {
			diffRange = args[1]
		}
		targetType = domain.TargetDiff
		revision, digest, err = resolveDiffTarget(deps.Root, diffRange)
	case "--commit":
		if len(args) != 2 {
			fail("--commit requiere un SHA o referencia")
		}
		targetType = domain.TargetCommit
		revision, digest, err = resolveCommitTarget(deps.Root, args[1])
	case "--file":
		if len(args) != 2 {
			fail("--file requiere una ruta")
		}
		targetType = domain.TargetFileSet
		revision, digest, scope, err = resolveFileTarget(deps.Root, args[1])
	case "status":
		cmdReviewStatus(deps, args[1:])
		return
	case "history":
		cmdReviewHistory(deps, args[1:])
		return
	case "show":
		cmdReviewShow(deps, args[1:])
		return
	default:
		fail("subcomando de review desconocido: %s\n"+
			"uso: mem review --diff [rango] | --commit <sha> | --file <ruta>\n"+
			"     mem review status [<review-id>] | history [--limit N] | show <review-id>", args[0])
	}
	if err != nil {
		fail("resolver target: %v", err)
	}
	review, err := usecases.StartReview(deps.ReviewRepo, usecases.StartReviewInput{
		Project: deps.Project, TargetType: targetType, Revision: revision, Digest: digest, Scope: scope,
	})
	if err != nil {
		fail("iniciar revisión: %v", err)
	}
	fmt.Println(review.ID)
	fmt.Printf("target_digest: %s\n", review.Target.Digest())
	fmt.Printf("independence: %s", review.IndependenceLevel)
	if review.IndependenceReason != "" {
		fmt.Printf(" (%s)", review.IndependenceReason)
	}
	fmt.Println()
}

func resolveCommitTarget(root, ref string) (string, string, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", ref+"^{commit}")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("commit %q no existe", ref)
	}
	sha := strings.TrimSpace(string(out))
	return sha, sha, nil
}

func resolveDiffTarget(root, diffRange string) (string, string, error) {
	args := []string{"diff", "--binary"}
	revision := "working-tree"
	if diffRange != "" {
		args = append(args, diffRange)
		revision = diffRange
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("git diff %q: %w", diffRange, err)
	}
	sum := sha256.Sum256(append([]byte("diff\x00"+revision+"\x00"), out...))
	return revision, hex.EncodeToString(sum[:]), nil
}

func resolveFileTarget(root, requested string) (string, string, []string, error) {
	abs := requested
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, requested)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", "", nil, err
	}
	var paths []string
	if info.IsDir() {
		err = filepath.WalkDir(abs, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return "", "", nil, err
		}
	} else {
		paths = []string{abs}
	}
	sort.Strings(paths)
	hash := sha256.New()
	scope := make([]string, 0, len(paths))
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return "", "", nil, err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", "", nil, err
		}
		scope = append(scope, filepath.ToSlash(rel))
		hash.Write([]byte("file\x00" + filepath.ToSlash(rel) + "\x00"))
		hash.Write(data)
		hash.Write([]byte{0})
	}
	revision := filepath.ToSlash(requested)
	return revision, hex.EncodeToString(hash.Sum(nil)), scope, nil
}
