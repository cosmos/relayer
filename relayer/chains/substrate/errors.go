package substrate

const (
	ErrTextConsensusStateNotFound                   = "consensus state not found: %s"
	ErrTextSubstrateDoesNotHaveQueryForTransactions = "substrate chains do not support transaction querying"
	ErrParachainSetNotFound                         = "parachain set not found"
	ErrAuthoritySetNotFound                         = "authority set not found"
	ErrDifferentTypesOfCallsMixed                   = "different types of calls can't be mixed"
)
