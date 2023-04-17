package icon

func (m *ConsensusState) ValidateBasic() error { return nil }
func (m *ConsensusState) ClientType() string   { return "icon" }
func (m *ConsensusState) GetTimestamp() uint64 { return 0 }
