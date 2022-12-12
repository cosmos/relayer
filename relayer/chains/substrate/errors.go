package substrate

const (
	ErrTextConsensusStateNotFound                   = "consensus state not found: %s"
	ErrTextSubstrateDoesNotHaveQueryForTransactions = "substrate chains do not support transaction querying"
	ErrDifferentTypesOfCallsMixed                   = "different types of calls can't be mixed"
)
