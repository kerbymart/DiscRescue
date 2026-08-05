package coordinator

import "testing"

func TestBuildShutdownPlanCancelMatchesTDDOrder(t *testing.T) {
	plan, err := BuildShutdownPlan(ShutdownCancel)
	if err != nil {
		t.Fatalf("build shutdown plan: %v", err)
	}

	want := []ShutdownStep{
		ShutdownStopScheduling,
		ShutdownCancelWorker,
		ShutdownDrainImage,
		ShutdownCommitMap,
		ShutdownFinalCheckpoint,
		ShutdownCatalogBestEffort,
		ShutdownCloseImageMap,
		ShutdownCloseCatalog,
		ShutdownStopWorker,
		ShutdownStopLogWriter,
		ShutdownNotifyTUI,
		ShutdownRestoreTerminal,
	}
	assertShutdownSteps(t, plan.Steps, want)
}

func TestBuildShutdownPlanPausePreservesTerminalRestorationAndSkipsFullStop(t *testing.T) {
	plan, err := BuildShutdownPlan(ShutdownPause)
	if err != nil {
		t.Fatalf("build shutdown plan: %v", err)
	}

	if containsShutdownStep(plan.Steps, ShutdownCloseCatalog) {
		t.Fatal("pause shutdown plan should not close the catalog")
	}
	if containsShutdownStep(plan.Steps, ShutdownStopLogWriter) {
		t.Fatal("pause shutdown plan should not stop the log writer")
	}
	if !containsShutdownStep(plan.Steps, ShutdownRestoreTerminal) {
		t.Fatal("pause shutdown plan must restore the terminal")
	}
}

func TestBuildShutdownPlanFatalErrorKeepsCatalogBestEffortBeforeTerminalRestore(t *testing.T) {
	plan, err := BuildShutdownPlan(ShutdownFatalError)
	if err != nil {
		t.Fatalf("build shutdown plan: %v", err)
	}

	catalogIndex := shutdownStepIndex(plan.Steps, ShutdownCatalogBestEffort)
	restoreIndex := shutdownStepIndex(plan.Steps, ShutdownRestoreTerminal)
	if catalogIndex == -1 || restoreIndex == -1 {
		t.Fatalf("expected catalog and terminal steps in fatal plan: %+v", plan.Steps)
	}
	if catalogIndex >= restoreIndex {
		t.Fatalf("expected catalog best-effort step before terminal restore, got %+v", plan.Steps)
	}
}

func TestBuildShutdownPlanRejectsUnknownReason(t *testing.T) {
	if _, err := BuildShutdownPlan(ShutdownReason("unknown")); err == nil {
		t.Fatal("expected unknown shutdown reason to fail")
	}
}

func assertShutdownSteps(t *testing.T, got, want []ShutdownStep) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("unexpected shutdown step count: got %d want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("unexpected shutdown step at %d: got %s want %s", index, got[index], want[index])
		}
	}
}

func containsShutdownStep(steps []ShutdownStep, target ShutdownStep) bool {
	return shutdownStepIndex(steps, target) >= 0
}

func shutdownStepIndex(steps []ShutdownStep, target ShutdownStep) int {
	for index, step := range steps {
		if step == target {
			return index
		}
	}
	return -1
}
