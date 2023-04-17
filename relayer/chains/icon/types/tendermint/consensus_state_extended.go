package tendermint

func (m *ConsensusState) ValidateBasic() error { return nil }
func (m *ConsensusState) ClientType() string   { return "icon" }
