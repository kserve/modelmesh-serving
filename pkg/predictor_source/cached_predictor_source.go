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
	"container/heap"
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"

	"github.com/go-logr/logr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	EVENT_UPDATE = iota
	EVENT_DELETE
)

var _ PredictorRegistry = (*cachedPredictorSource)(nil)

type EventType int

// Both UPDATE and DELETE events are expected to have a Predictor containing the corresponding resourceVersion
type PredictorStreamEvent struct {
	EventType EventType
	Predictor *api.Predictor
}

type PredictorEventStream chan PredictorStreamEvent

type PredictorStatusUpdater interface {
	// UpdateStatus updates the given Predictor's status conditional on its resource version.
	// The returned boolean reflects whether the conditional update was successful; in both
	// cases the returned Predictor will reflect the latest version from the source *including
	// its updated resource version*.
	// In the case that the Predictor is not found, (nil, rv, false, nil) will be returned, where
	// rv is the current resource version at which the Predictor was observed to not be found.
	UpdateStatus(ctx context.Context, p *api.Predictor) (newP *api.Predictor, resourceVersion string, ok bool, err error)
}

func newCachedPredictorSource(id string, name string,
	updater PredictorStatusUpdater) *cachedPredictorSource {
	return &cachedPredictorSource{
		sourceId:   id,
		sourceName: name,
		updater:    updater,
		cache:      make(map[types.NamespacedName]*api.Predictor),
		lock:       &sync.RWMutex{},
		logger:     ctrl.Log.WithName(fmt.Sprintf("PS-%s", name)), //TODO logger TBD
	}
}

// cachedPredictorSource implements PredictorRegistry. Embedders are expected to implement
// the StartWatch() function for a complete PredictorSource implementation.
type cachedPredictorSource struct {
	sourceId   string
	sourceName string
	started    bool
	updater    PredictorStatusUpdater

	cache map[types.NamespacedName]*api.Predictor
	lock  *sync.RWMutex

	// Event processing goroutine unlocks whenever a remote call or
	// blocking channel operation is made
	lockHeld bool

	// Predictors in the cache with deletion time in the past.
	// These are pruned after 20 seconds to avoid race condition
	// with UpdateStatus
	deletionQueue predictorDeletionHeap

	logger logr.Logger
}

// ----- PredictorSource and PredictorRegistry function implementations

func (s *cachedPredictorSource) GetSourceId() string {
	return s.sourceId
}

func (s *cachedPredictorSource) GetSourceName() string {
	return s.sourceName
}

func (s *cachedPredictorSource) Get(_ context.Context, name types.NamespacedName) (*api.Predictor, error) {
	if p := s.get(name); p != nil && !isDeleted(p) {
		return p, nil
	}
	return nil, nil
}

func (s *cachedPredictorSource) Find(_ context.Context, namespace string, predicate func(*api.Predictor) bool) (bool, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	for _, p := range s.cache {
		if !isDeleted(p) && p.Namespace == namespace && predicate(p) {
			return true, nil
		}
	}
	return false, nil
}

func (s *cachedPredictorSource) UpdateStatus(ctx context.Context, predictor *api.Predictor) (bool, error) {
	if predictor == nil {
		return false, errors.New("can't update with nil predictor")
	}
	name := nn(predictor)
	if existing := s.get(name); existing == nil || predictor.ResourceVersion != existing.ResourceVersion {
		return false, nil
	}
	newPredictor, rv, ok, err := s.updater.UpdateStatus(ctx, predictor)
	if err != nil {
		return false, err
	}
	if newPredictor == nil {
		// assert !ok && rv != ""
		ok = false
		newPredictor = deletedPredictor(name, rv)
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	s.offerToCache(name, newPredictor)
	return ok, nil
}

// -------- Private reciever functions

func (s *cachedPredictorSource) start() error {
	if s.started {
		return errors.New("already started")
	}
	s.started = true
	return nil
}

// called only from event-processing goroutine
func (s *cachedPredictorSource) unlock() {
	// assert s.lockHeld
	s.lock.Unlock()
	s.lockHeld = false
}

// called only from event-processing goroutine
func (s *cachedPredictorSource) readEvent(inChan chan PredictorStreamEvent) (PredictorStreamEvent, bool) {
	if s.lockHeld {
		if psep, _ := pollChan(inChan); psep != nil {
			return *psep, true
		}
		s.pruneDeletions()
		s.unlock()
	}
	pse, ok := <-inChan
	return pse, ok
}

// called only from event-processing goroutine
func (s *cachedPredictorSource) writeEvent(c chan<- PredictorEvent, pep *PredictorEvent) {
	pe := *pep
	if s.lockHeld {
		if ok := offerChan(c, pe); ok {
			return // processed ok
		}
		s.unlock()
	}
	c <- pe
}

// called only from event-processing goroutine
func (s *cachedPredictorSource) processEvent(p *api.Predictor, et EventType) *PredictorEvent {
	if p == nil {
		s.logger.Error(nil, "Encountered unexpected nil Predictor event")
		return nil
	}
	name := nn(p)
	if et == EVENT_DELETE && p.DeletionTimestamp != nil {
		p.DeletionTimestamp = &metav1.Time{}
	}
	if !s.lockHeld {
		s.lock.Lock()
		s.lockHeld = true
	}
	if s.offerToCache(name, p) {
		return (*PredictorEvent)(&name)
	}
	return nil
}

// called only from event-processing goroutine while lock is held
func (s *cachedPredictorSource) pruneDeletions() {
	//assert s.lockHeld
	if len(s.deletionQueue) != 0 {
		cutoff := time.Now().Add(20 * time.Second)
		for {
			p := s.deletionQueue.PopExpired(cutoff)
			if p == nil {
				return
			}
			name := nn(p)
			if current := s.cache[name]; current == p {
				delete(s.cache, name)
			}
		}
	}
}

func (s *cachedPredictorSource) get(name types.NamespacedName) *api.Predictor {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.cache[name]
}

// must be called with write lock
func (s *cachedPredictorSource) offerToCache(name types.NamespacedName, p *api.Predictor) bool {
	current := s.cache[name]
	if newer, err := compareResourceVersion(current, p); !newer {
		if err != nil {
			s.logger.Error(err, "Error comparing resource versions prior to cache update", "predictor", name)
		} else {
			s.logger.Info("Ignoring predictor update or delete because cache has more recent", "predictor", name)
		}
		return false
	}
	if p.DeletionTimestamp != nil {
		twoSecsAgo := secondsAgo(2)
		if p.DeletionTimestamp.Before(twoSecsAgo) {
			p.DeletionTimestamp = twoSecsAgo
		}
		if current.DeletionTimestamp == nil {
			heap.Push(&s.deletionQueue, p)
		}
	}
	s.cache[name] = p
	return true
}

// ------ Non-reciever utility functions

func nn(p *api.Predictor) types.NamespacedName {
	return types.NamespacedName{Namespace: p.Namespace, Name: p.Name}
}

// deletedPredictor returns a struct representing a deleted Predictor at a given resourceVersion
func deletedPredictor(name types.NamespacedName, resourceVersion string) *api.Predictor {
	return &api.Predictor{
		TypeMeta: metav1.TypeMeta{Kind: "Predictor", APIVersion: "serving.kserve.io/v1alpha1"},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         name.Namespace,
			Name:              name.Name,
			ResourceVersion:   resourceVersion,
			DeletionTimestamp: &metav1.Time{},
		},
	}
}

func isDeleted(p *api.Predictor) bool {
	return p.DeletionTimestamp != nil && time.Now().After(p.DeletionTimestamp.Time)
}

func secondsAgo(secs int) *metav1.Time {
	return &metav1.Time{Time: time.Now().Add(time.Duration(-secs) * time.Second)}
}

// compareResourceVersion returns true if new Predictor's version is newer than provided version
func compareResourceVersion(existing *api.Predictor, new *api.Predictor) (bool, error) {
	newRV, err := strconv.ParseInt(new.ResourceVersion, 10, 64)
	if err != nil {
		return false, fmt.Errorf("unexpected resource version encountered: %s", new.ResourceVersion)
	}
	if existing == nil {
		return true, nil
	}
	if existRV, err := strconv.ParseInt(existing.ResourceVersion, 10, 64); err != nil {
		return false, fmt.Errorf("unexpected resource version encountered: %s", existing.ResourceVersion)
	} else {
		return newRV > existRV, nil
	}
}

// pollChan performs a non-blocking channel read
func pollChan(c <-chan PredictorStreamEvent) (*PredictorStreamEvent, bool) {
	select {
	case e, ok := <-c:
		if !ok {
			return nil, false
		}
		return &e, true
	default:
		return nil, true
	}
}

// offerChan performs a non-blocking channel write
func offerChan(c chan<- PredictorEvent, pe PredictorEvent) bool {
	select {
	case c <- pe:
		return true
	default:
		return false
	}
}
