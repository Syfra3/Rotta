package workflow

const LifecycleModelID = "rotta.lifecycle/v1"

// LifecycleModelInstructions is the runtime-owned lifecycle authority shared by generated roles.
func LifecycleModelInstructions() string {
	return `## Lifecycle Authority — ` + LifecycleModelID + `

- The source/runtime ` + "`" + LifecycleModelID + "`" + ` model is the only lifecycle authority.
- Canonical feature paths are ` + "`.rotta/current/manifest.yaml`" + `, ` + "`.rotta/current/state.yaml`" + `, and ` + "`.rotta/current/evidence/`" + `.
- Only the Rotta-Orchestrator may approve, checkpoint, archive, recover, or complete a feature.
- Every other role returns bounded evidence only and cannot activate retired state-machine assets or root evidence paths.
`
}
