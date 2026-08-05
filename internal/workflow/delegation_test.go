package workflow

import "testing"

func TestSCN734_735_OrchestratorRequiresBoundedWorkerEvidence(t *testing.T) {
	if err := ValidateWorkerEvidence(WorkerEvidence{}); err == nil {
		t.Fatal("want evidence-less worker rejection")
	}
}
