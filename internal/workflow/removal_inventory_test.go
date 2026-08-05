package workflow

import "testing"

func TestSCN731_742_InventoryBlocksMixedAndIncompleteRemoval(t *testing.T) {
	discovered := []InventoryEntry{{Path: "legacy/runner.go", Category: "workflow_code"}, {Path: "testdata/v1.yaml", Category: "fixture"}}
	if err := ValidateRemovalInventory(discovered, discovered[:1]); err == nil {
		t.Fatal("want incomplete inventory")
	}
	mixed := []InventoryEntry{{Path: "legacy/runner.go", Category: "workflow_code", Role: "mixed", Reachability: "used", Classification: "mixed", Disposition: "delete", PostRemovalPlan: "test"}, {Path: "testdata/v1.yaml", Category: "fixture", Role: "v1", Reachability: "used", Classification: "legacy_only", Disposition: "remove", PostRemovalPlan: "test"}}
	if err := ValidateRemovalInventory(discovered, mixed); err == nil {
		t.Fatal("want mixed deletion rejection")
	}
}
