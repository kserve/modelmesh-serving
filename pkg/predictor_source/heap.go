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
	"time"

	api "github.com/kserve/modelmesh-serving/apis/serving/v1alpha1"
)

var _ heap.Interface = (*predictorDeletionHeap)(nil)

// predictorDeletionHeap maintains a queue of Predictor deletions ordered by DeletionTimestamp
type predictorDeletionHeap []*api.Predictor

func (h predictorDeletionHeap) Len() int { return len(h) }
func (h predictorDeletionHeap) Less(i, j int) bool {
	return h[i].DeletionTimestamp.Before(h[j].DeletionTimestamp)
}
func (h predictorDeletionHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *predictorDeletionHeap) Push(v interface{}) {
	p := v.(*api.Predictor)
	if p.DeletionTimestamp != nil {
		*h = append(*h, p)
	}
}

func (h *predictorDeletionHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (h predictorDeletionHeap) Peek() *api.Predictor {
	if len(h) != 0 {
		return h[0]
	}
	return nil
}

func (h *predictorDeletionHeap) PopExpired(cutoff time.Time) *api.Predictor {
	pdh := *h
	if len(pdh) != 0 && pdh[0].DeletionTimestamp.Time.Before(cutoff) {
		return heap.Pop(h).(*api.Predictor)
	}
	return nil
}
