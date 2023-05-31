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
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const namespace = "test-namespace"

// Mock repository
type testWatcher struct {
	lock     sync.Mutex
	registry map[types.NamespacedName]api.Predictor
	events   []api.Predictor
	curRev   int
	// Number of historical revisions simulated as being available. Attempting
	// to watch from an earlier revision will fail
	compactAge   int
	disconnected bool

	watchChans []PredictorEventStream
}

func (t *testWatcher) disconnect() {
	t.lock.Lock()
	defer t.lock.Unlock()
	if !t.disconnected {
		for _, c := range t.watchChans {
			close(c)
		}
		t.watchChans = nil
		t.disconnected = true
	}
}

func (t *testWatcher) reconnect() {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.disconnected = false
}

func (t *testWatcher) set(p *api.Predictor) {
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.registry == nil {
		t.registry = make(map[types.NamespacedName]api.Predictor)
	}
	t.curRev += 1
	p.ResourceVersion = strconv.Itoa(t.curRev)
	pp := *p
	t.registry[nn(p)] = pp
	t.events = append(t.events, pp)
	for _, c := range t.watchChans {
		c <- event(pp)
	}
}

func (t *testWatcher) delete(nn types.NamespacedName) {
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.registry == nil {
		return
	}
	p, ok := t.registry[nn]
	if !ok {
		return
	}
	delete(t.registry, nn)
	t.curRev += 1
	p.ResourceVersion = strconv.Itoa(t.curRev)
	p.DeletionGracePeriodSeconds = &[]int64{-1}[0]
	t.events = append(t.events, p)
	for _, c := range t.watchChans {
		c <- event(p)
	}
}

func (t *testWatcher) UpdateStatus(ctx context.Context, p *api.Predictor) (newP *api.Predictor, resourceVersion string, ok bool, err error) {
	nn := nn(p)
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.disconnected {
		return nil, "", false, errors.New("repository disconnected")
	}
	exist, ok := t.registry[nn]
	if !ok {
		return nil, strconv.Itoa(t.curRev), false, nil
	}
	if exist.ResourceVersion != p.ResourceVersion {
		return &exist, strconv.Itoa(t.curRev), false, nil
	}
	t.curRev += 1
	pp := *p
	pp.ResourceVersion = strconv.Itoa(t.curRev)
	t.registry[nn] = pp
	t.events = append(t.events, pp)
	for _, c := range t.watchChans {
		c <- event(pp)
	}
	return &pp, pp.ResourceVersion, true, nil
}

func (t *testWatcher) Refresh(ctx context.Context, limit int, from string) (api.PredictorList, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.disconnected {
		return api.PredictorList{}, errors.New("repository disconnected")
	}
	pl := make([]api.Predictor, 0, len(t.registry))
	for _, p := range t.registry {
		pl = append(pl, p)
	}
	return api.PredictorList{
		//		TypeMeta: metav1.TypeMeta{},
		ListMeta: metav1.ListMeta{
			ResourceVersion: strconv.Itoa(t.curRev),
		},
		Items: pl,
	}, nil
}

func event(p api.Predictor) PredictorStreamEvent {
	var et EventType = EVENT_UPDATE
	if p.DeletionGracePeriodSeconds != nil && *p.DeletionGracePeriodSeconds == -1 {
		et = EVENT_DELETE
	}
	return PredictorStreamEvent{
		EventType: et,
		Predictor: &p,
	}
}

func (t *testWatcher) Watch(ctx context.Context, resourceVersion string) (PredictorEventStream, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.disconnected {
		return nil, errors.New("repository disconnected")
	}
	rv, err := strconv.Atoi(resourceVersion)
	if err != nil {
		return nil, err
	}
	if t.curRev-rv > t.compactAge {
		return nil, ERR_TOO_OLD
	}
	s := make(PredictorEventStream, 128)
	// Not handling case where buffer is too small here
	for i := range t.events {
		p := &t.events[i]
		prv, _ := strconv.Atoi(p.ResourceVersion)
		if prv >= rv {
			s <- event(*p)
		}
	}
	t.watchChans = append(t.watchChans, s)
	return s, nil
}

func makeTestWatcher() *testWatcher {
	tw := testWatcher{compactAge: 3}
	tw.set(makePredictor(1))
	return &tw
}

func makePredictor(id int) *api.Predictor {
	return &api.Predictor{
		TypeMeta: metav1.TypeMeta{Kind: "Predictor", APIVersion: api.GroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("testPredictor%d", id),
			Namespace: namespace,
		},
		Spec: api.PredictorSpec{
			Model: api.Model{
				Type: api.ModelType{
					Name: "tensorflow",
				},
				Path: fmt.Sprintf("testModel%d", id),
			},
		},
		Status: api.PredictorStatus{
			ActiveModelState: api.Pending,
		},
	}
}

var _ = Describe("WatchRefresh-based PredictorSource", func() {
	var err error
	var ps PredictorSource

	tw := makeTestWatcher()

	It("Should create watcher-based PredictorSource correctly", func() {
		ps = NewWatchPredictorSource("test", "TestPredictor", tw)

		Expect(ps).ToNot(BeNil())
		Expect(ps.GetSourceId()).To(Equal("test"))
	})

	var pr PredictorRegistry
	var pec PredictorEventChan

	It("Should start watcher-based PredictorSource correctly", func() {
		cxt, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		pr, pec, err = ps.StartWatch(cxt)
		Expect(err).ToNot(HaveOccurred())
		Expect(pr).ToNot(BeNil())
		Expect(pec).ToNot(BeNil())
		Expect(pr.GetSourceName()).To(Equal("TestPredictor"))
	})

	It("Should contain existing initial registry state", func() {
		count := 0
		found, err := pr.Find(context.TODO(), namespace, func(p *api.Predictor) bool {
			count += 1
			return false
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(found).ToNot(BeTrue())
		Expect(count).To(Equal(1))
		m := collectEvents(pec, 1, 500)
		Expect(m).To(HaveLen(1))
		Expect(m).To(HaveKey(pe("testPredictor1")))
		Expect(pec).To(BeEmpty())
	})

	It("Should fail to update nonexistent Predictor", func() {
		ok, err := pr.UpdateStatus(context.TODO(), &api.Predictor{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "notexist",
				Namespace:       namespace,
				ResourceVersion: "123",
			},
		})
		Expect(ok).ToNot(BeTrue())
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should fail to update out-of-date Predictor", func() {
		ok, err := pr.UpdateStatus(context.TODO(), &api.Predictor{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "testPredictor1",
				Namespace:       namespace,
				ResourceVersion: "-1",
			},
		})
		Expect(ok).ToNot(BeTrue())
		Expect(err).ToNot(HaveOccurred())
	})

	latestRv := 1

	updateRoundTripTest := func() {
		Expect(pec).To(BeEmpty())

		p1 := makePredictor(1)
		p1.ResourceVersion = strconv.Itoa(latestRv)
		p1.Status.ActiveModelState = api.Loaded
		p1.Status.Available = true

		ok, err := pr.UpdateStatus(context.TODO(), p1)
		Expect(ok).To(BeTrue())
		Expect(err).ToNot(HaveOccurred())

		Expect(err).ToNot(HaveOccurred())
		m := collectEvents(pec, 1, 500)
		// There's an expected race condition here where updating may or
		// may not produce a reconcile event (we don't really care about
		// this event since we're the one who just did the update)
		Expect(len(m) <= 1).To(BeTrue())
		if len(m) == 1 {
			Expect(m).To(HaveKey(pe("testPredictor1")))
		}
		p, err := pr.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: "testPredictor1"})
		Expect(err).ToNot(HaveOccurred())
		Expect(p.Status.ActiveModelState).To(Equal(api.Loaded))
		Expect(p.ResourceVersion).To(Equal(strconv.Itoa(latestRv + 1)))
		latestRv += 1
	}

	It("Should succeed to update new Predictor", updateRoundTripTest)

	It("Should succeed to update new Predictor following watch disconnect", func() {
		tw.disconnect()
		tw.reconnect()
		updateRoundTripTest()

		tw.set(makePredictor(2))
		Expect(collectEvents(pec, 1, 500)).To(HaveLen(1))
	})

	It("Should fail to observe new Predictor while disconnected", func() {
		tw.disconnect()
		tw.set(makePredictor(3))
		tw.set(makePredictor(4))

		m := collectEvents(pec, 0, 500)
		Expect(m).To(BeEmpty())
		p, err := pr.Get(context.TODO(), nn2("testPredictor3"))
		Expect(p).To(BeNil())
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should observe new Predictors after reconnecting", func() {
		tw.reconnect()

		m := collectEvents(pec, 2, 6000)
		Expect(m).To(HaveLen(2))
		Expect(m).ToNot(HaveKey(pe("testPredictor1"))) // Older events shouldn't be received
		Expect(m).To(HaveKey(pe("testPredictor3")))
		Expect(m).To(HaveKey(pe("testPredictor4")))
		p, err := pr.Get(context.TODO(), nn2("testPredictor3"))
		Expect(p).ToNot(BeNil())
		Expect(err).ToNot(HaveOccurred())
		p, err = pr.Get(context.TODO(), nn2("testPredictor4"))
		Expect(p).ToNot(BeNil())
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should observe expected events and registry state after reconnecting even if history is lost", func() {
		tw.disconnect()

		// Make a bunch of registry changes while disconnected. This corresponds to many more events than
		// the test repo's compaction age is set to (3), so a forced refresh will need to happen upon reconnect.
		tw.delete(nn2("testPredictor3"))
		for i := 5; i <= 20; i++ {
			tw.set(makePredictor(i))
		}
		update := makePredictor(4)
		update.Spec.Model.Path = "newPath"
		tw.set(update)

		// Verify that we did not observe updates yet
		m := collectEvents(pec, 0, 500)
		Expect(m).To(BeEmpty())
		p, err := pr.Get(context.TODO(), nn2("testPredictor10"))
		Expect(p).To(BeNil())
		Expect(err).ToNot(HaveOccurred())
		p, err = pr.Get(context.TODO(), nn2("testPredictor3"))
		Expect(p).ToNot(BeNil())
		Expect(err).ToNot(HaveOccurred())

		// Reconnect and make sure expected updates are made

		tw.reconnect()

		m = collectEvents(pec, 18, 6000)
		Expect(m).To(HaveLen(16 + 1 + 1)) // 16 additions, 1 deletion, 1 update
		Expect(m).To(HaveKey(pe("testPredictor10")))
		Expect(m).To(HaveKey(pe("testPredictor3")))
		Expect(m).To(HaveKey(pe("testPredictor4")))
		Expect(m).To(HaveKey(pe("testPredictor20")))
		p, err = pr.Get(context.TODO(), nn2("testPredictor10"))
		Expect(p).ToNot(BeNil())
		Expect(err).ToNot(HaveOccurred())
		p, err = pr.Get(context.TODO(), nn2("testPredictor2"))
		Expect(p).ToNot(BeNil())
		Expect(err).ToNot(HaveOccurred())
		// This one was updated
		p, err = pr.Get(context.TODO(), nn2("testPredictor4"))
		Expect(p).ToNot(BeNil())
		Expect(p.Spec.Model.Path).To(Equal("newPath"))
		Expect(err).ToNot(HaveOccurred())
		// This one was deleted
		p, err = pr.Get(context.TODO(), nn2("testPredictor3"))
		Expect(p).To(BeNil())
		Expect(err).ToNot(HaveOccurred())
	})

})

// Helpers

// We collect events in a set because duplicate reconcile events are "allowed"
func collectEvents(c PredictorEventChan, expected int, timeoutMillis int) map[PredictorEvent]bool {
	m := make(map[PredictorEvent]bool)
	dl := time.Now().Add(time.Duration(timeoutMillis) * time.Millisecond)
	for len(m) < expected {
		select {
		case pe := <-c:
			m[pe] = true
		case <-time.After(time.Until(dl)):
			return m
		}
	}
	select {
	case pe := <-c:
		m[pe] = true
	case <-time.After(300 * time.Millisecond):
	}

	return m
}

func pe(pname string) PredictorEvent {
	return PredictorEvent(nn2(pname))
}

func nn2(pname string) types.NamespacedName {
	return types.NamespacedName{
		Namespace: namespace,
		Name:      pname,
	}
}
