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
	"time"

	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"

	"k8s.io/apimachinery/pkg/types"
)

type Error string

func (e Error) Error() string { return string(e) }

const ERR_TOO_OLD = Error("ERR_TOO_OLD")

type PredictorWatcher interface {
	// UpdateStatus updates the given Predictor's status conditional on its resource version.
	// The returned boolean reflects whether the conditional update was successful; in both
	// cases the returned Predictor will reflect the latest version from the source *including
	// its updated resource version*.
	// In the case that the Predictor is not found, (nil, rv, false, nil) will be returned, where
	// rv is the current resource version at which the Predictor was observed to not be found.
	UpdateStatus(ctx context.Context, p *api.Predictor) (newP *api.Predictor, resourceVersion string, ok bool, err error)

	// Refresh returns a complete list of Predictors at the current point in time as of the
	// ResourceVersion contained in the returned PredictorList.
	// The limit arg may be used to request a limit, and the from arg can be used to request
	// a continuation list from a previously returned trucated list (the returned lists'
	// ResourceVersions will match in this case).
	Refresh(ctx context.Context, limit int, from string) (api.PredictorList, error)

	// Watch for predictor events starting from the specified resourceVersion. A returned error
	// of ERR_TOO_OLD indicates that the requested resourceVersion is too old. In this case a refresh/resync
	// is typically performed.
	Watch(ctx context.Context, resourceVersion string) (PredictorEventStream, error)
}

// NewWatchPredictorSource returns a new PredictorSource based on a refresh-and-watch PredictorWatcher implementation
func NewWatchPredictorSource(id string, name string, watcher PredictorWatcher) PredictorSource {
	return &watchPredictorSource{
		cachedPredictorSource: *newCachedPredictorSource(id, name, watcher),
		watcher:               watcher,
	}
}

type watchPredictorSource struct {
	cachedPredictorSource
	watcher PredictorWatcher
}

var _ PredictorSource = (*watchPredictorSource)(nil)

func (w *watchPredictorSource) StartWatch(ctx context.Context) (PredictorRegistry, PredictorEventChan, error) {
	if err := w.start(); err != nil {
		return nil, nil, err
	}
	w.logger.Info("Starting PredictorSource event watch")

	list, err := w.watcher.Refresh(ctx, 0, "")
	if err != nil {
		//TODO tbc
		w.started = false
		return nil, nil, err
	}
	w.logger.Info("Completed initial refresh request",
		"numPredictors", len(list.Items), "resourceVersion", list.ResourceVersion)
	resourceVersion := list.ResourceVersion
	outChan := make(PredictorEventChan, 128) //TODO size TBD configurable
	var initialEvents []PredictorEvent
	// initial cache population
	for i := range list.Items {
		p := &list.Items[i]
		name := nn(p)
		w.cache[name] = p
		if initialEvents == nil {
			if offerChan(outChan, PredictorEvent(name)) {
				continue // added to channel
			}
			initialEvents = make([]PredictorEvent, cap(outChan)) // overflow for full channel
		}
		initialEvents = append(initialEvents, PredictorEvent(name))
	}
	list.Items = nil

	go func() {
		for i := range initialEvents {
			outChan <- initialEvents[i]
		}
		initialEvents = nil
		for {
			if w.lockHeld {
				w.unlock()
			}
			// let's start a watch
			var inChan PredictorEventStream
			for {
				// watch from the same resource version
				w.logger.Info("Initiating watch", "fromResourceVersion", resourceVersion)
				if inChan, err = w.watcher.Watch(context.Background(), resourceVersion); err == nil {
					break
				}
				if err == ERR_TOO_OLD {
					w.logger.Info("Received ERR_TOO_OLD from Watch attempt, refreshing cache")
					// resourceVersion too old, let's refresh then retry watch with new resourceVersion
					resourceVersion = w.refreshCache(ctx, outChan)
				} else {
					w.logger.Error(err, "Watch failed, retrying in 5 seconds")
					time.Sleep(5 * time.Second) //TODO back-off retry delay
				}
			}
			// watch established ok
			for {
				pse, ok := w.readEvent(inChan)
				if !ok {
					break // Input channel closed, attempt to re-establish watch
				}
				if pep := w.processEvent(pse.Predictor, pse.EventType); pep != nil {
					w.writeEvent(outChan, pep)
					resourceVersion = pse.Predictor.ResourceVersion //TODO compare probably .. should never decrease
				}
			}
		}
	}()

	return w, outChan, nil
}

// returns new resource version
func (w *watchPredictorSource) refreshCache(ctx context.Context, outChan chan<- PredictorEvent) string {
	// assert !w.lockHeld
	var list api.PredictorList
	for {
		var err error
		if list, err = w.watcher.Refresh(ctx, 0, ""); err == nil {
			break
		}
		w.logger.Error(err, "Refresh failed, retrying in 5 seconds")
		time.Sleep(5 * time.Second) //TODO back-off retry delay
	}
	//TODO maybe compare list.ResourceVersion with resourceVersion here & log warning
	present := make(map[types.NamespacedName]struct{}, len(list.Items))
	for i := range list.Items {
		p := &list.Items[i]
		if pep := w.processEvent(p, EVENT_UPDATE); pep != nil {
			w.writeEvent(outChan, pep)
		}
		present[nn(p)] = struct{}{}
	}
	list.Items = nil
	if !w.lockHeld {
		w.lock.RLock()
	}
	var toDelete []types.NamespacedName
	for nn := range w.cache {
		if _, ok := present[nn]; !ok {
			toDelete = append(toDelete, nn)
		}
	}
	if !w.lockHeld {
		w.lock.RUnlock()
	}
	if len(toDelete) != 0 {
		for i := range toDelete {
			if pep := w.processEvent(deletedPredictor(toDelete[i], list.ResourceVersion), EVENT_DELETE); pep != nil {
				w.writeEvent(outChan, pep)
			}
		}
	}
	return list.ResourceVersion
}
