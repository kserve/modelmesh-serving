// Copyright 2021 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package predictor_source

import (
	"context"
	"fmt"
	"time"
)

// NewPredictorStreamSource returns a new PredictorSource based on a persistent event stream
func NewPredictorStreamSource(id string, name string,
	events PredictorEventStream, updater PredictorStatusUpdater) PredictorSource {
	return &streamPredictorSource{
		cachedPredictorSource: *newCachedPredictorSource(id, name, updater),
		inChan:                events,
	}
}

type streamPredictorSource struct {
	cachedPredictorSource
	inChan PredictorEventStream
}

var _ PredictorSource = (*streamPredictorSource)(nil)

func (s *streamPredictorSource) StartWatch(ctx context.Context) (PredictorRegistry, PredictorEventChan, error) {
	if err := s.start(); err != nil {
		return nil, nil, err
	}
	s.logger.Info("Starting PredictorSource event watch")

	outChan := make(PredictorEventChan, 128) //TODO size TBD configurable
	var initialEvents []PredictorEvent
	for {
		select {
		case e, ok := <-s.inChan:
			if !ok {
				return nil, nil, fmt.Errorf("event channel closed") //TODO tbd
			}
			if pep := s.processEvent(e.Predictor, e.EventType); pep != nil {
				pe := *pep
				if initialEvents == nil {
					if offerChan(outChan, pe) {
						continue // added to channel
					}
					initialEvents = make([]PredictorEvent, cap(outChan)) // overflow for full channel
				}
				initialEvents = append(initialEvents, pe)
			}
			continue
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
			// break
		}
		break
	}

	go func() {
		for i := range initialEvents {
			outChan <- initialEvents[i]
		}
		initialEvents = nil
		for {
			pse, ok := s.readEvent(s.inChan)
			if !ok {
				s.logger.Info("PredictorEventChan closed")
				return
			}
			if pep := s.processEvent(pse.Predictor, pse.EventType); pep != nil {
				s.writeEvent(outChan, pep)
			}
		}
	}()

	return s, outChan, nil
}
