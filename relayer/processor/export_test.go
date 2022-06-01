package processor

// Test use only
func (pp *PathProcessor) PathEnd1Messages(message string) SequenceCache {
	return pp.pathEnd1.messageCache[message]
}

// Test use only
func (pp *PathProcessor) PathEnd2Messages(message string) SequenceCache {
	return pp.pathEnd2.messageCache[message]
}
