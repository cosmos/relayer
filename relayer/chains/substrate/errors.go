package substrate

const (
	ErrTextConsensusStateNotFound                   = "consensus state not found: %s"
	ErrTextSubstrateDoesNotHaveQueryForTransactions = "substrate chains do not support transaction querying"
	ErrBeefyAttributesNotFound                      = "beefy authorities not found"
	ErrBeefyConstructNotFound                       = "beefy construct not found"
	ErrDifferentTypesOfCallsMixed                   = "different types of calls can't be mixed"
)
