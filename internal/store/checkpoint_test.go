package store

import (
	"testing"
)

func TestEnqueueCheckpointJob(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Auth System", "token auth")
	ws, _ := s.OpenWorkSession("auth-system", "sess-1")

	job, err := s.EnqueueCheckpointJob(CheckpointJobInput{
		WorkSessionID:         ws.ID,
		FeatureID:             "auth-system",
		Reason:                "stop",
		TriggerType:           "auto",
		TranscriptStartOffset: 0,
		TranscriptEndOffset:   1024,
		SemanticText:          "discussed auth token design",
		MechanicalFacts:       MechanicalFacts{FilesEdited: []FileEdit{{Path: "auth.go", Count: 2}}},
	})
	if err != nil {
		t.Fatalf("EnqueueCheckpointJob: %v", err)
	}
	if job.Status != "queued" {
		t.Errorf("Status = %q, want %q", job.Status, "queued")
	}
	if job.FeatureID != "auth-system" {
		t.Errorf("FeatureID = %q, want %q", job.FeatureID, "auth-system")
	}
}

func TestDequeueCheckpointJob(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Auth System", "token auth")
	ws, _ := s.OpenWorkSession("auth-system", "sess-1")

	s.EnqueueCheckpointJob(CheckpointJobInput{
		WorkSessionID:         ws.ID,
		FeatureID:             "auth-system",
		Reason:                "stop",
		TranscriptStartOffset: 0,
		TranscriptEndOffset:   512,
		SemanticText:          "some text",
		MechanicalFacts:       MechanicalFacts{},
	})

	job, err := s.DequeueCheckpointJob()
	if err != nil {
		t.Fatalf("DequeueCheckpointJob: %v", err)
	}
	if job == nil {
		t.Fatal("expected a job, got nil")
	}
	if job.Status != "running" {
		t.Errorf("Status = %q, want %q", job.Status, "running")
	}

	// Second dequeue should return nil (no more queued jobs)
	job2, _ := s.DequeueCheckpointJob()
	if job2 != nil {
		t.Error("expected nil on second dequeue")
	}
}

func TestCompleteCheckpointJob(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Auth System", "token auth")
	ws, _ := s.OpenWorkSession("auth-system", "sess-1")

	enqueued, _ := s.EnqueueCheckpointJob(CheckpointJobInput{
		WorkSessionID:         ws.ID,
		FeatureID:             "auth-system",
		Reason:                "stop",
		TranscriptStartOffset: 0,
		TranscriptEndOffset:   512,
		SemanticText:          "text",
		MechanicalFacts:       MechanicalFacts{},
	})

	s.DequeueCheckpointJob()
	err := s.CompleteCheckpointJob(enqueued.ID, nil)
	if err != nil {
		t.Fatalf("CompleteCheckpointJob: %v", err)
	}

	job, _ := s.GetCheckpointJob(enqueued.ID)
	if job.Status != "done" {
		t.Errorf("Status = %q, want %q", job.Status, "done")
	}
}

func TestFailCheckpointJobRequeuesUnderMaxRetries(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Auth System", "token auth")
	ws, _ := s.OpenWorkSession("auth-system", "sess-1")

	enqueued, _ := s.EnqueueCheckpointJob(CheckpointJobInput{
		WorkSessionID:         ws.ID,
		FeatureID:             "auth-system",
		Reason:                "stop",
		TranscriptStartOffset: 0,
		TranscriptEndOffset:   512,
		SemanticText:          "text",
		MechanicalFacts:       MechanicalFacts{},
	})

	// Fail 3 times — each should re-queue (retries 1, 2, 3)
	for i := 1; i <= maxCheckpointRetries; i++ {
		s.DequeueCheckpointJob()
		err := s.FailCheckpointJob(enqueued.ID, "api timeout")
		if err != nil {
			t.Fatalf("FailCheckpointJob (retry %d): %v", i, err)
		}

		job, _ := s.GetCheckpointJob(enqueued.ID)
		if job.Status != "queued" {
			t.Errorf("retry %d: Status = %q, want %q", i, job.Status, "queued")
		}
		if job.RetryCount != i {
			t.Errorf("retry %d: RetryCount = %d, want %d", i, job.RetryCount, i)
		}
		if job.Error != "api timeout" {
			t.Errorf("retry %d: Error = %q, want %q", i, job.Error, "api timeout")
		}
	}

	// 4th failure — should stay failed (exceeded max retries)
	s.DequeueCheckpointJob()
	err := s.FailCheckpointJob(enqueued.ID, "final failure")
	if err != nil {
		t.Fatalf("FailCheckpointJob (final): %v", err)
	}

	job, _ := s.GetCheckpointJob(enqueued.ID)
	if job.Status != "failed" {
		t.Errorf("final: Status = %q, want %q", job.Status, "failed")
	}
	if job.Error != "final failure" {
		t.Errorf("final: Error = %q, want %q", job.Error, "final failure")
	}
	if job.RetryCount != maxCheckpointRetries {
		t.Errorf("final: RetryCount = %d, want %d", job.RetryCount, maxCheckpointRetries)
	}
}

func TestCompleteCheckpointJobWithError(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Auth System", "token auth")
	ws, _ := s.OpenWorkSession("auth-system", "sess-1")

	enqueued, _ := s.EnqueueCheckpointJob(CheckpointJobInput{
		WorkSessionID:         ws.ID,
		FeatureID:             "auth-system",
		Reason:                "stop",
		TranscriptStartOffset: 0,
		TranscriptEndOffset:   512,
		SemanticText:          "text",
		MechanicalFacts:       MechanicalFacts{},
	})

	s.DequeueCheckpointJob()
	errMsg := "api timeout"
	err := s.CompleteCheckpointJob(enqueued.ID, &errMsg)
	if err != nil {
		t.Fatalf("CompleteCheckpointJob with error: %v", err)
	}

	// Should be re-queued (first failure, under max retries)
	job, _ := s.GetCheckpointJob(enqueued.ID)
	if job.Status != "queued" {
		t.Errorf("Status = %q, want %q", job.Status, "queued")
	}
	if job.RetryCount != 1 {
		t.Errorf("RetryCount = %d, want 1", job.RetryCount)
	}
}

func TestAddCheckpointObservation(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Auth System", "token auth")
	ws, _ := s.OpenWorkSession("auth-system", "sess-1")

	job, _ := s.EnqueueCheckpointJob(CheckpointJobInput{
		WorkSessionID:         ws.ID,
		FeatureID:             "auth-system",
		Reason:                "stop",
		TranscriptStartOffset: 0,
		TranscriptEndOffset:   512,
		SemanticText:          "text",
		MechanicalFacts:       MechanicalFacts{},
	})

	obs, err := s.AddCheckpointObservation(CheckpointObservationInput{
		CheckpointJobID: job.ID,
		WorkSessionID:   ws.ID,
		FeatureID:       "auth-system",
		Kind:            "summary",
		PayloadJSON:     `{"goals": ["implement refresh tokens"]}`,
		SummaryText:     "Discussed token refresh design. Decided to use rotating refresh tokens.",
	})
	if err != nil {
		t.Fatalf("AddCheckpointObservation: %v", err)
	}
	if obs.Kind != "summary" {
		t.Errorf("Kind = %q, want %q", obs.Kind, "summary")
	}
}

func TestGetObservationsForWorkSession(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Auth System", "token auth")
	ws, _ := s.OpenWorkSession("auth-system", "sess-1")

	job, _ := s.EnqueueCheckpointJob(CheckpointJobInput{
		WorkSessionID: ws.ID, FeatureID: "auth-system", Reason: "stop",
		TranscriptStartOffset: 0, TranscriptEndOffset: 512,
		SemanticText: "text", MechanicalFacts: MechanicalFacts{},
	})

	s.AddCheckpointObservation(CheckpointObservationInput{
		CheckpointJobID: job.ID, WorkSessionID: ws.ID, FeatureID: "auth-system",
		Kind: "summary", SummaryText: "First checkpoint",
	})
	s.AddCheckpointObservation(CheckpointObservationInput{
		CheckpointJobID: job.ID, WorkSessionID: ws.ID, FeatureID: "auth-system",
		Kind: "blocker", SummaryText: "Need API key for external service",
	})

	obs, err := s.GetObservationsForWorkSession(ws.ID)
	if err != nil {
		t.Fatalf("GetObservationsForWorkSession: %v", err)
	}
	if len(obs) != 2 {
		t.Fatalf("got %d observations, want 2", len(obs))
	}
}

func TestCheckpointIdempotency(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Auth System", "token auth")
	ws, _ := s.OpenWorkSession("auth-system", "sess-1")

	input := CheckpointJobInput{
		WorkSessionID:         ws.ID,
		FeatureID:             "auth-system",
		Reason:                "stop",
		TranscriptStartOffset: 0,
		TranscriptEndOffset:   512,
		SemanticText:          "text",
		MechanicalFacts:       MechanicalFacts{},
	}

	job1, _ := s.EnqueueCheckpointJob(input)
	job2, _ := s.EnqueueCheckpointJob(input)

	if job1.ID != job2.ID {
		t.Errorf("expected idempotent enqueue, got IDs %d and %d", job1.ID, job2.ID)
	}
}

func TestGetMechanicalFactsForWorkSession(t *testing.T) {
	s := openTestStore(t)
	s.AddFeature("Auth System", "token auth")
	ws, _ := s.OpenWorkSession("auth-system", "sess-1")

	facts1 := MechanicalFacts{
		FilesEdited: []FileEdit{{Path: "auth.go", Count: 2}},
		Commits:     []CommitFact{{Hash: "abc123", Message: "add auth"}},
	}
	facts2 := MechanicalFacts{
		FilesEdited: []FileEdit{{Path: "middleware.go", Count: 1}},
		TestRuns:    []TestRunFact{{Command: "go test ./...", Passed: true}},
	}

	s.EnqueueCheckpointJob(CheckpointJobInput{
		WorkSessionID: ws.ID, FeatureID: "auth-system", Reason: "stop",
		TranscriptStartOffset: 0, TranscriptEndOffset: 512,
		SemanticText: "text", MechanicalFacts: facts1,
	})
	s.EnqueueCheckpointJob(CheckpointJobInput{
		WorkSessionID: ws.ID, FeatureID: "auth-system", Reason: "stop",
		TranscriptStartOffset: 512, TranscriptEndOffset: 1024,
		SemanticText: "more text", MechanicalFacts: facts2,
	})

	merged, err := s.GetMechanicalFactsForWorkSession(ws.ID)
	if err != nil {
		t.Fatalf("GetMechanicalFactsForWorkSession: %v", err)
	}
	if len(merged.FilesEdited) != 2 {
		t.Errorf("FilesEdited = %d, want 2", len(merged.FilesEdited))
	}
	if len(merged.Commits) != 1 {
		t.Errorf("Commits = %d, want 1", len(merged.Commits))
	}
	if len(merged.TestRuns) != 1 {
		t.Errorf("TestRuns = %d, want 1", len(merged.TestRuns))
	}
}
