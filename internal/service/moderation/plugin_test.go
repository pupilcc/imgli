package moderation

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type stubChecker struct {
	name string
	res  CheckResult
	err  error
}

func (s stubChecker) Name() string { return s.name }
func (s stubChecker) Check(context.Context, ImageRef) (CheckResult, error) {
	return s.res, s.err
}

func TestDecideBlockBeatsReview(t *testing.T) {
	s1, s2 := 0.5, 0.9
	d := Decide([]CheckResult{
		{Plugin: "a", Severity: SeverityReview, Score: &s1},
		{Plugin: "b", Severity: SeverityBlock, Score: &s2},
	}, DefaultPolicy())
	if d.Status != "rejected" || !d.Flagged {
		t.Fatalf("status=%s flagged=%v, want rejected true", d.Status, d.Flagged)
	}
	if d.Score == nil || *d.Score != 0.9 {
		t.Fatalf("score=%v, want 0.9", d.Score)
	}
}

func TestDecideReviewPending(t *testing.T) {
	s := 0.8
	d := Decide([]CheckResult{
		{Plugin: "nsfwjs", Severity: SeverityReview, Score: &s},
	}, DefaultPolicy())
	if d.Status != "pending" || !d.Flagged {
		t.Fatalf("got status=%s flagged=%v", d.Status, d.Flagged)
	}
}

func TestDecideAllNoneNormal(t *testing.T) {
	s := 0.1
	d := Decide([]CheckResult{
		{Plugin: "nsfwjs", Severity: SeverityNone, Score: &s},
	}, DefaultPolicy())
	if d.Status != "normal" || d.Flagged {
		t.Fatalf("got status=%s flagged=%v", d.Status, d.Flagged)
	}
	if d.Score == nil || *d.Score != 0.1 {
		t.Fatalf("score=%v", d.Score)
	}
}

func TestRunPipelineSingleErrorPropagates(t *testing.T) {
	_, err := RunPipeline(context.Background(), ImageRef{}, []Checker{
		stubChecker{name: "x", err: errors.New("boom")},
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("err=%v, want boom", err)
	}
}

func TestRunPipelineMultiErrorFailOpen(t *testing.T) {
	s := 0.2
	results, err := RunPipeline(context.Background(), ImageRef{}, []Checker{
		stubChecker{name: "bad", err: errors.New("boom")},
		stubChecker{name: "ok", res: CheckResult{Plugin: "ok", Severity: SeverityNone, Score: &s}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Plugin != "ok" {
		t.Fatalf("results=%+v", results)
	}
}

func TestThresholdCheckerMapsReview(t *testing.T) {
	sc := &stubScorer{score: 0.8}
	c := WrapScorer("nsfwjs", sc, 0.75, SeverityReview)
	r, err := c.Check(context.Background(), ImageRef{
		MIME: "image/png",
		Open: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("x")), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Severity != SeverityReview || r.Score == nil || *r.Score != 0.8 {
		t.Fatalf("got %+v", r)
	}
}

func TestThresholdCheckerBelowNone(t *testing.T) {
	sc := &stubScorer{score: 0.1}
	c := WrapScorer("nsfwjs", sc, 0.75, SeverityReview)
	r, err := c.Check(context.Background(), ImageRef{
		Open: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("x")), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Severity != SeverityNone {
		t.Fatalf("severity=%s", r.Severity)
	}
}

type stubScorer struct{ score float64 }

func (s *stubScorer) Score(context.Context, io.Reader, string, string) (float64, error) {
	return s.score, nil
}

func TestBuildCheckersDisabledEmpty(t *testing.T) {
	cfg := DefaultConfig()
	if cs := BuildCheckersFromConfig(cfg); len(cs) != 0 {
		t.Fatalf("len=%d", len(cs))
	}
}

func TestBuildCheckersEnabledOne(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Provider = "nsfwjs"
	cfg.Endpoint = "http://127.0.0.1:9/"
	cfg.Threshold = 0.75
	cfg.Action = "pending"
	cs := BuildCheckersFromConfig(cfg)
	if len(cs) != 1 || cs[0].Name() != "nsfwjs" {
		t.Fatalf("%+v", cs)
	}
}

func TestPolicyFromConfigRejectedMapsBlockPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Action = "rejected"
	pol := PolicyFromConfig(cfg)
	s := 0.9
	d := Decide([]CheckResult{{Severity: SeverityBlock, Score: &s}}, pol)
	if d.Status != "rejected" {
		t.Fatalf("status=%s", d.Status)
	}
	// 投影 overThreshold=block；若误用 review 也应对齐 rejected
	d2 := Decide([]CheckResult{{Severity: SeverityReview, Score: &s}}, pol)
	if d2.Status != "rejected" {
		t.Fatalf("review path status=%s, want rejected", d2.Status)
	}
}

func TestOverThresholdFromAction(t *testing.T) {
	if overThresholdFromAction("pending") != SeverityReview {
		t.Fatal("pending → review")
	}
	if overThresholdFromAction("rejected") != SeverityBlock {
		t.Fatal("rejected → block")
	}
}
