// Copyright 2026 The Ratel Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package inproc

import "math/rand"

// CompleteGrudge computes a directed partition map where each node is isolated
// from every node outside its own group.
func CompleteGrudge(groups [][]string) map[string][]string {
	universe := make(map[string]struct{})
	for _, group := range groups {
		for _, node := range group {
			universe[node] = struct{}{}
		}
	}

	grudge := make(map[string][]string)
	for _, group := range groups {
		groupSet := make(map[string]struct{}, len(group))
		for _, node := range group {
			groupSet[node] = struct{}{}
		}
		for _, node := range group {
			blocked := make([]string, 0, len(universe)-len(group))
			for other := range universe {
				if _, ok := groupSet[other]; !ok {
					blocked = append(blocked, other)
				}
			}
			grudge[node] = blocked
		}
	}
	return grudge
}

// ShuffleNodes returns a shuffled copy of nodes.
func ShuffleNodes(nodes []string, rng *rand.Rand) []string {
	out := append([]string(nil), nodes...)
	rng.Shuffle(len(out), func(i, j int) {
		out[i], out[j] = out[j], out[i]
	})
	return out
}

// RandomHalvesGrudge returns the Jepsen "parts" topology for nodes using the
// provided RNG.
func RandomHalvesGrudge(nodes []string, rng *rand.Rand) map[string][]string {
	shuffled := ShuffleNodes(nodes, rng)
	split := len(shuffled) / 2
	return CompleteGrudge([][]string{shuffled[:split], shuffled[split:]})
}

// MajoritiesRingGrudge returns the Jepsen "majority-ring" topology for a
// particular node order.
func MajoritiesRingGrudge(nodes []string) map[string][]string {
	universe := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		universe[node] = struct{}{}
	}

	n := len(nodes)
	majority := n/2 + 1
	mid := majority / 2
	grudge := make(map[string][]string, n)

	for i := range nodes {
		window := make(map[string]struct{}, majority)
		for j := 0; j < majority; j++ {
			window[nodes[(i+j)%n]] = struct{}{}
		}
		center := nodes[(i+mid)%n]
		blocked := make([]string, 0, len(universe)-len(window))
		for other := range universe {
			if _, ok := window[other]; !ok {
				blocked = append(blocked, other)
			}
		}
		grudge[center] = blocked
	}
	return grudge
}
