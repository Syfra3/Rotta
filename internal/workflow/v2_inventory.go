package workflow

import "errors"

type V2InventoryEntry struct{ Path, Category, Role, Reachability, Classification, Disposition, Replacement, Tests, PostRemovalPlan string }

func ValidateV2RemovalInventory(discovered []V2InventoryEntry, inventory []V2InventoryEntry) error {
	if len(discovered) != len(inventory) {
		return errors.New("v2 removal inventory is incomplete: unreconciled discovered candidate")
	}
	for _, candidate := range discovered {
		found := false
		for _, entry := range inventory {
			if entry.Path == candidate.Path && entry.Category == candidate.Category {
				found = true
				if entry.Role == "" || entry.Reachability == "" || entry.Classification == "" || entry.Disposition == "" || entry.PostRemovalPlan == "" {
					return errors.New("v2 removal inventory is incomplete: required evidence field missing")
				}
				if entry.Classification == "mixed" && entry.Disposition == "delete" {
					return errors.New("v2 removal inventory forbids deleting mixed asset")
				}
			}
		}
		if !found {
			return errors.New("v2 removal inventory is incomplete: unreconciled discovered candidate")
		}
	}
	return nil
}
