package substrate

const (
	ErrTextConsensusStateNotFound                   = "consensus state not found: %s"
	ErrTextSubstrateDoesNotHaveQueryForTransactions = "substrate chains do not support transaction querying"
	ErrBeefyAttributesNotFound                      = "beefy authorities not found: storage key %v, paraids %v, block hash %v"
	ErrBeefyConstructNotFound                       = "beefy construct not found: storage key %v, authorities %v, block hash %v"
)
