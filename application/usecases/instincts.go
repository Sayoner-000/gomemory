package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"mem/application/ports"
	"mem/domain"
)

type Instincts struct {
	Queue   ports.ObservationQueueRepository
	Repo    ports.InstinctRepository
	Global  ports.InstinctGlobalRepository
	Project string
	// Ephemeral marca un proyecto con raíz en el directorio temporal: lee los
	// instincts globales, pero no cuenta como proyecto para promoverlos.
	Ephemeral bool
	Now       func() time.Time
	NewID     func() string
	Memories  ports.MemoryRepository
}

func (u *Instincts) Evolve(ctx context.Context, g ports.SocraticGenerator, client, slug, template string) (string, string, error) {
	if g == nil || !g.Available() {
		return "", "", ports.ErrNoGenerator
	}
	if u.Repo == nil {
		return "", "", errors.New("instinct repository unavailable")
	}
	list, err := u.Repo.List(ctx, true)
	if err != nil {
		return "", "", err
	}
	var chosen *domain.Instinct
	for idx := range list {
		if list[idx].Slug == slug {
			chosen = &list[idx]
			break
		}
	}
	if chosen == nil {
		return "", "", errors.New("instinct not found")
	}
	safeSlug := strings.ReplaceAll(slug, ":", "-")
	for _, r := range safeSlug {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return "", "", errors.New("invalid instinct slug")
		}
	}
	name := "instinct-" + safeSlug
	prompt := strings.NewReplacer("{{name}}", name, "{{tools}}", strings.Join(domain.AllMCPTools(), ", "), "{{instinct}}", domain.ProtectText(chosen.Description)).Replace(template)
	flow := domain.RefinementFlow{Client: client, Project: u.Project, Mode: domain.ModeRefine, OriginalPrompt: prompt, Questions: []domain.Question{}, Answers: []domain.Answer{}}
	result, err := g.Rewrite(ctx, ports.SocraticGenerationRequest{Flow: flow, Context: "Devuelve una skill completa en proposed, no un prompt para crearla."})
	if err != nil {
		return "", "", err
	}
	body := domain.ProtectText(result.Proposed)
	if errs := domain.ValidateSkillBody(name, body, domain.AllMCPTools()); len(errs) > 0 {
		return "", "", fmt.Errorf("invalid skill: %s", strings.Join(errs, "; "))
	}
	return name, body, nil
}

func (u *Instincts) now() time.Time {
	if u.Now != nil {
		return u.Now()
	}
	return time.Now()
}
func (u *Instincts) Observe(ctx context.Context, sig domain.ExpandSignals) error {
	if u.Queue == nil {
		return nil
	}
	sample := sig.Command
	kind := "cmd"
	key := ""
	if sig.Event == domain.ExpandPrompt {
		kind = "correction"
		sample = sig.Prompt
		key = domain.CorrectionKey(sample)
	} else if sig.Tool == domain.ToolShell {
		key = domain.ObservationKey(sample)
	}
	if key == "" {
		return nil
	}
	sample = domain.TruncateRunes(domain.StripTerminalControls(domain.RedactSecrets(domain.RedactPrivate(sample))), 200)
	var id string
	if u.NewID != nil {
		id = u.NewID()
	} else {
		v, err := uuid.NewV7()
		if err != nil {
			return err
		}
		id = v.String()
	}
	return u.Queue.Append(ctx, domain.Observation{ID: id, T: u.now(), Kind: kind, Key: key, Sample: sample})
}
func (u *Instincts) Fold(ctx context.Context) error { return u.FoldPending(ctx, u.now()) }
func (u *Instincts) FoldPending(ctx context.Context, now time.Time) error {
	if u.Queue == nil || u.Repo == nil {
		return nil
	}
	batch, err := u.Queue.ReadBatch(ctx, 500)
	if err != nil {
		return err
	}
	if len(batch) > 0 {
		if err := u.Repo.Fold(ctx, batch, now); err != nil {
			return err
		}
	}
	ids := make([]string, 0, len(batch))
	for _, obs := range batch {
		ids = append(ids, obs.ID)
	}
	removeErr := u.Queue.Remove(ctx, ids)
	if err := u.Repo.PurgeFolded(ctx, now.Add(-7*24*time.Hour)); err != nil {
		return errors.Join(removeErr, err)
	}
	if u.Global != nil && !u.Ephemeral {
		all, err := u.Repo.ForContext(ctx, .5)
		if err != nil {
			return errors.Join(removeErr, err)
		}
		for _, i := range all {
			if err := u.Global.RecordProject(ctx, u.Project, i, now); err != nil {
				return errors.Join(removeErr, err)
			}
		}
	}
	return removeErr
}

func (u *Instincts) List(ctx context.Context, all bool) ([]domain.Instinct, error) {
	if u.Repo == nil {
		return nil, errors.New("instinct repository unavailable")
	}
	list, err := u.Repo.List(ctx, true)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, i := range list {
		seen[i.Slug] = true
	}
	if u.Global != nil {
		global, err := u.Global.GlobalFor(ctx, .5)
		if err != nil {
			return nil, err
		}
		for _, g := range global {
			found := false
			for idx := range list {
				if list[idx].Slug == g.Slug {
					list[idx].Scope = "global"
					list[idx].Projects = g.Projects
					found = true
					break
				}
			}
			if !found && !seen[g.Slug] {
				list = append(list, g)
			}
		}
	}
	out := []domain.Instinct{}
	for _, i := range list {
		if all || i.Status != "rejected" {
			out = append(out, i)
		}
	}
	return out, nil
}

func (u *Instincts) Confirm(ctx context.Context, slug string) error {
	if u.Repo == nil || u.Memories == nil {
		return errors.New("instinct confirmation unavailable")
	}
	return u.review(ctx, slug, "confirmed", func(i domain.Instinct) error {
		// Un slug antiguo aún sin sanear no debe llevar su credencial al
		// topic_key, que no pasa por la redacción de la memoria (B-001).
		safe := domain.ProtectObservationKey(slug)
		m := domain.Memory{Project: u.Project, Type: domain.Preference, Title: "Instinct: " + safe, Content: i.Description, TopicKey: "instinct:" + safe}
		_, _, err := SaveWithGate(u.Memories, &m)
		return err
	})
}

// Reject marca el instinct como rechazado en este proyecto; si era una
// propuesta global, el rechazo local impide que se inyecte aquí.
func (u *Instincts) Reject(ctx context.Context, slug string) error {
	if u.Repo == nil {
		return errors.New("instinct repository unavailable")
	}
	return u.review(ctx, slug, "rejected", nil)
}

// review busca el instinct en el proyecto y, si no está, entre las
// propuestas globales que List muestra: una propuesta global visible aquí
// también debe poder confirmarse o rechazarse aquí (B-001).
func (u *Instincts) review(ctx context.Context, slug, status string, before func(domain.Instinct) error) error {
	local, err := u.Repo.List(ctx, true)
	if err != nil {
		return err
	}
	var found domain.Instinct
	ok, adopt := false, false
	for _, i := range local {
		if i.Slug == slug {
			found, ok = i, true
			break
		}
	}
	if !ok && u.Global != nil {
		global, err := u.Global.GlobalFor(ctx, .5)
		if err != nil {
			return err
		}
		for _, g := range global {
			if g.Slug == slug {
				found, ok, adopt = g, true, true
				found.Status = "proposed"
				break
			}
		}
	}
	if !ok {
		return errors.New("instinct not found")
	}
	if !domain.CanSetInstinctStatus(found.Status, status) {
		return errors.New("instinct status is final")
	}
	if before != nil {
		if err := before(found); err != nil {
			return err
		}
	}
	if adopt {
		return u.Repo.Adopt(ctx, found, status, u.now())
	}
	return u.Repo.SetStatus(ctx, slug, status)
}

// InstinctContext merges project and global proposals while honoring local rejection.
func InstinctContext(ctx context.Context, repo ports.InstinctRepository, global ports.InstinctGlobalRepository) ([]domain.Instinct, error) {
	all := []domain.Instinct{}
	blocked := map[string]bool{}
	seen := map[string]bool{}
	if repo != nil {
		local, err := repo.List(ctx, true)
		if err != nil {
			return nil, err
		}
		for _, i := range local {
			if i.Status == "rejected" {
				blocked[i.Slug] = true
				continue
			}
			if i.Confidence+1e-9 >= .7 {
				all = append(all, i)
				seen[i.Slug] = true
			}
		}
	}
	if global != nil {
		list, err := global.GlobalFor(ctx, .7)
		if err != nil {
			return nil, err
		}
		for _, i := range list {
			if !blocked[i.Slug] && !seen[i.Slug] {
				all = append(all, i)
				seen[i.Slug] = true
			}
		}
	}
	if len(all) > 8 {
		all = all[:8]
	}
	for idx := range all {
		all[idx].Description = strings.ReplaceAll(domain.ProtectText(all[idx].Description), "\n", " ")
	}
	return all, nil
}
