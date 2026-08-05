package workflow

import "errors"

type InventoryEntry struct{ Path, Category, Role, Reachability, Classification, Disposition, Replacement, Tests, PostRemovalPlan string }

func ValidateRemovalInventory(discovered []InventoryEntry, inventory []InventoryEntry) error {
	if len(discovered) != len(inventory) {
		return errors.New(" removal inventory is incomplete: unreconciled discovered candidate")
	}
	for _, candidate := range discovered {
		found := false
		for _, entry := range inventory {
			if entry.Path == candidate.Path && entry.Category == candidate.Category {
				found = true
				if entry.Role == "" || entry.Reachability == "" || entry.Classification == "" || entry.Disposition == "" || entry.PostRemovalPlan == "" {
					return errors.New(" removal inventory is incomplete: required evidence field missing")
				}
				if entry.Classification == "mixed" && entry.Disposition == "delete" {
					return errors.New(" removal inventory forbids deleting mixed asset")
				}
			}
		}
		if !found {
			return errors.New(" removal inventory is incomplete: unreconciled discovered candidate")
		}
	}
	return nil
}
