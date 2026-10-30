package main_test

import (
	"os/exec"
	"testing"
)

func TestHCD001EvaluationCapture(t *testing.T) {
	cmd := exec.Command("python3", "../../scripts/hcd001_evaluate.py", "--self-test")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("captura no conforme: %v\n%s", err, output)
	}
}
