package doctorsearch

import (
	"context"

	"github.com/rs/zerolog/log"
)

type indexControl struct {
	indexes            [2]*nGramsIndex
	numInFlightQueries [2]int64

	maxConcurrentQueries int64

	acquireChan    chan *nGramsIndex
	releaseChan    chan *nGramsIndex
	signalNewIndex chan *nGramsIndex
}

func NewIndexControl(maxConcurrentQueries int) indexControl {
	ic := indexControl{
		acquireChan:          make(chan *nGramsIndex),
		releaseChan:          make(chan *nGramsIndex),
		signalNewIndex:       make(chan *nGramsIndex, 1),
		maxConcurrentQueries: int64(maxConcurrentQueries),
	}
	go ic.start()
	return ic
}

func (ic *indexControl) start() {
	const (
		CurrentIndex = 0
		OldIndex     = 1
	)

	var (
		totalInFlightQueries int64
		acqChan              = ic.acquireChan
	)
	for {
		totalInFlightQueries = ic.numInFlightQueries[CurrentIndex] + ic.numInFlightQueries[OldIndex]
		if totalInFlightQueries >= ic.maxConcurrentQueries {
			// A nil channel is never ready, so this effectively switches off
			// the "acquire case", acting as a limit on the number of outstanding
			// queries at any one time.
			acqChan = nil
		} else {
			acqChan = ic.acquireChan
		}
		select {
		case acqChan <- ic.indexes[CurrentIndex]:
			ic.numInFlightQueries[CurrentIndex] += 1
		case releasedIndex := <-ic.releaseChan:
			if releasedIndex == nil {
				continue
			} else if releasedIndex == ic.indexes[CurrentIndex] {
				ic.numInFlightQueries[CurrentIndex] -= 1
			} else {
				ic.numInFlightQueries[OldIndex] -= 1
				// If no more queries are in-flight on the old index,
				// it means we can safely signal clean it up.
				if ic.numInFlightQueries[OldIndex] == 0 {
					ic.cleanupOldIndex()
				}
			}
		case newIndex := <-ic.signalNewIndex:
			if ic.indexes[OldIndex] != nil {
				// TOOD: kind of a logic/temporal error, this shouldnt happen
				// reject the index, too bad...
				continue
			}

			ic.indexes[OldIndex], ic.indexes[CurrentIndex] = ic.indexes[CurrentIndex], newIndex
			ic.numInFlightQueries[OldIndex] = ic.numInFlightQueries[CurrentIndex]

			log.Info().Msg("switched active index")

			// If no queries are outstanding on the old index, we can clean it up straight away.
			if ic.numInFlightQueries[OldIndex] == 0 {
				ic.cleanupOldIndex()
			}
		}
	}
}

func (ic *indexControl) cleanupOldIndex() {
	const OldIndex = 1

	oldIndex := ic.indexes[OldIndex]
	if oldIndex != nil {
		oldIndex.Close()
	}
	ic.indexes[OldIndex] = nil
	ic.numInFlightQueries[OldIndex] = 0

	log.Info().Msg("cleaned-up old index")
}

func (ic *indexControl) UseIndex(index *nGramsIndex) {
	ic.signalNewIndex <- index
}

func (ic *indexControl) Acquire(ctx context.Context) (*nGramsIndex, error) {
	select {
	case ix := <-ic.acquireChan:
		return ix, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (ic *indexControl) Release(index *nGramsIndex) {
	ic.releaseChan <- index
}
