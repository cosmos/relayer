package relayer

type ChainProcessor struct {
	chain                   *Chain
	unrelayedSequencesQueue chan uint64
}

const unrelayedSequencesQueueCapacity = 1000

func NewChainProcessor(chain *Chain) *ChainProcessor {
	return &ChainProcessor{
		chain:                   chain,
		unrelayedSequencesQueue: make(chan uint64, unrelayedSequencesQueueCapacity),
	}
}

func (p *ChainProcessor) Start() {
	go p.unrelayedSequencesWorker()
}

func (p *ChainProcessor) unrelayedSequencesWorker() {
	for unrelayedSequence := range p.unrelayedSequencesQueue {
		p.processUnrelayedSequence(unrelayedSequence)
	}
}

func (p *ChainProcessor) EnqueueJob(unrelayedSequences []uint64) {
	for _, unrelayedSequence := range unrelayedSequences {
		p.unrelayedSequencesQueue <- unrelayedSequence
	}
}

func (p *ChainProcessor) processUnrelayedSequence(unrelayedSequence uint64) {
	// TODO
}
