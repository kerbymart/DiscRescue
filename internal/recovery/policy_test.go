package recovery

import "testing"

func TestPolicyRegistryDefinesDistinctMethodSemantics(t *testing.T) {
	fast, err := PolicyForMethod(RecoveryMethodFast)
	if err != nil {
		t.Fatal(err)
	}
	balanced, err := PolicyForMethod(RecoveryMethodBalanced)
	if err != nil {
		t.Fatal(err)
	}
	gentle, err := PolicyForMethod(RecoveryMethodGentle)
	if err != nil {
		t.Fatal(err)
	}
	if fast.Fast.BlockSectors != 64 || len(fast.Adaptive) != 1 || fast.Targeted.Enabled || fast.FinalizeUnresolved {
		t.Fatalf("unexpected fast policy: %+v", fast)
	}
	if balanced.Fast.BlockSectors != 64 || len(balanced.Adaptive) != 2 || !balanced.Targeted.Enabled || !balanced.FinalizeUnresolved {
		t.Fatalf("unexpected balanced policy: %+v", balanced)
	}
	if gentle.Fast.BlockSectors != 32 || gentle.Adaptive[0].BlockSectors != 8 || gentle.Targeted.AttemptsLimit != 3 || gentle.FinalizeUnresolved {
		t.Fatalf("unexpected gentle policy: %+v", gentle)
	}
}

func TestPolicyRegistryRejectsUnknownMethod(t *testing.T) {
	if _, err := PolicyForMethod(RecoveryMethod("aggressive")); err == nil {
		t.Fatal("expected unknown method to fail")
	}
}
