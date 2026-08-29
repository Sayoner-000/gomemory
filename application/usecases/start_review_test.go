package usecases

import (
	"testing"

	"mem/domain"
)

func TestStartReviewRejectsEmptyDigestWithoutPersisting(t *testing.T) {
	repo := newMemoryReviewRepository()
	_, err := StartReview(repo, StartReviewInput{Project: "proj", TargetType: domain.TargetDiff})
	if err == nil {
		t.Fatal("StartReview debe rechazar el digest vacío")
	}
	if len(repo.reviews) != 0 {
		t.Fatal("StartReview persistió una revisión inválida")
	}
}

func TestStartReviewFreezesTargetAndDegradesSameModel(t *testing.T) {
	repo := newMemoryReviewRepository()
	review, err := StartReview(repo, StartReviewInput{
		Project:    "proj",
		TargetType: domain.TargetDiff,
		Revision:   "working-tree",
		Digest:     "sha256:frozen",
		Scope:      []string{"domain/"},
		ReviewerA:  ReviewerIdentity{Provider: "openai", Model: "gpt-x"},
		ReviewerB:  ReviewerIdentity{Provider: "openai", Model: "gpt-x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if review.Target.Digest() != "sha256:frozen" {
		t.Fatalf("target no congelado: %q", review.Target.Digest())
	}
	if review.IndependenceLevel != domain.IndependenceDegraded || review.IndependenceReason != "same-model" {
		t.Fatalf("independencia = %s/%s", review.IndependenceLevel, review.IndependenceReason)
	}
	stored, _ := repo.GetReview("proj", review.ID)
	if stored == nil || stored.Status != domain.ReviewAwaitingReviewers {
		t.Fatalf("revisión no persistida en awaiting_reviewers: %#v", stored)
	}
}
