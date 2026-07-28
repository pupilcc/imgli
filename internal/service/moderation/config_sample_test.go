package moderation

import (
	"context"
	"errors"
	"testing"
)

func TestShouldEnqueueModerate(t *testing.T) {
	if !ShouldEnqueueModerate(true, 0, "abc") {
		t.Fatal("guest always enqueues")
	}
	if !ShouldEnqueueModerate(false, 1, "abc") {
		t.Fatal("rate 1 always")
	}
	if ShouldEnqueueModerate(false, 0, "abc") {
		t.Fatal("rate 0 never for login")
	}
	a := ShouldEnqueueModerate(false, 0.5, "stable-key-1")
	b := ShouldEnqueueModerate(false, 0.5, "stable-key-1")
	if a != b {
		t.Fatal("must be deterministic")
	}
}

func TestRunPipelineReviewOnError(t *testing.T) {
	c := &errChecker{name: "bad"}
	out, err := RunPipelineWithErrorPolicy(context.Background(), ImageRef{}, []Checker{c}, "review")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Severity != SeverityReview {
		t.Fatalf("got %+v", out)
	}
}

type errChecker struct{ name string }

func (e *errChecker) Name() string { return e.name }
func (e *errChecker) Check(ctx context.Context, img ImageRef) (CheckResult, error) {
	return CheckResult{}, errors.New("boom")
}
