package statemanager

import (
	"github.com/consensys/zkevm-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/zkevm-monorepo/prover/protocol/wizard"
	"github.com/consensys/zkevm-monorepo/prover/utils"
	"github.com/consensys/zkevm-monorepo/prover/zkevm/prover/statemanager/accumulator"
	"github.com/consensys/zkevm-monorepo/prover/zkevm/prover/statemanager/accumulatorsummary"
	"github.com/consensys/zkevm-monorepo/prover/zkevm/prover/statemanager/mimccodehash"
	"github.com/consensys/zkevm-monorepo/prover/zkevm/prover/statemanager/statesummary"
)

// StateManager is a collection of modules responsible for attesting the
// correctness of the state-transitions occuring in Linea w.r.t. to the
// arithmetization.
type StateManager struct {
	accumulator                 accumulator.Module
	accumulatorSummaryConnector accumulatorsummary.Module
	stateSummary                statesummary.Module
	mimcCodeHash                mimccodehash.Module
}

// Settings stores all the setting to construct a StateManager and is passed to
// the [NewStateManager] function. All the settings of the submodules are
// constructed based on this structure
type Settings struct {
	AccSettings      accumulator.Settings
	MiMCCodeHashSize int
}

// NewStateManager instantiate the [StateManager] module
func NewStateManager(comp *wizard.CompiledIOP, settings Settings) StateManager {

	sm := StateManager{
		stateSummary: statesummary.NewModule(comp, settings.stateSummarySize()),
		accumulator:  accumulator.NewModule(comp, settings.AccSettings),
		mimcCodeHash: mimccodehash.NewModule(comp, mimccodehash.Inputs{
			Name: "MiMCCodeHash",
			Size: settings.MiMCCodeHashSize,
		}),
	}

	sm.accumulatorSummaryConnector = *accumulatorsummary.NewModule(
		comp,
		accumulatorsummary.Inputs{
			Name:        "ACCUMULATOR_SUMMARY",
			Accumulator: sm.accumulator,
		},
	)

	sm.accumulatorSummaryConnector.ConnectToStateSummary(comp, &sm.stateSummary)
	sm.mimcCodeHash.ConnectToRom(comp, rom(comp), romLex(comp))
	sm.stateSummary.ConnectToHub(comp, acp(comp), scp(comp))
	lookupStateSummaryCodeHash(comp, &sm.stateSummary.Account, &sm.mimcCodeHash)

	return sm
}

// Assign assignes the submodules of the state-manager. It requires the
// arithmetization columns to be assigned first.
func (sm *StateManager) Assign(run *wizard.ProverRuntime, shomeiTraces [][]statemanager.DecodedTrace) {

	sm.stateSummary.Assign(run, shomeiTraces)
	sm.accumulator.Assign(run, utils.Join(shomeiTraces...))
	sm.accumulatorSummaryConnector.Assign(run)
	sm.mimcCodeHash.Assign(run)

}

// stateSummarySize returns the number of rows to give to the state-summary
// module.
func (s *Settings) stateSummarySize() int {
	return utils.NextPowerOfTwo(s.AccSettings.MaxNumProofs)
}
