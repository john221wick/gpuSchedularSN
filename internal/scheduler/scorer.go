package scheduler

type ScoredGroup struct {
	GPUIDs []int
	Score  float32
}

func Score(topo *Topology, numGPUs int, minVRAMMB uint64) *ScoredGroup {
	if topo == nil || numGPUs <= 0 {
		return nil
	}

	var eligible []int
	for i, d := range topo.Devices {
		free := d.VRAMTotalMB - d.VRAMUsedMB
		if free >= minVRAMMB {
			eligible = append(eligible, i)
		}
	}

	if len(eligible) < numGPUs {
		return nil
	}

	combos := combinations(eligible, numGPUs)

	var best *ScoredGroup
	for _, combo := range combos {
		var score float32
		for i := 0; i < len(combo); i++ {
			for j := i + 1; j < len(combo); j++ {
				score += topo.Bandwidth[combo[i]][combo[j]]
			}
		}
		if best == nil || score > best.Score {
			gpuIDs := make([]int, len(combo))
			for i, idx := range combo {
				gpuIDs[i] = topo.Devices[idx].ID
			}
			best = &ScoredGroup{GPUIDs: gpuIDs, Score: score}
		}
	}

	return best
}

func combinations(items []int, k int) [][]int {
	if k == 0 {
		return [][]int{{}}
	}
	if len(items) < k {
		return nil
	}

	var result [][]int

	// take first item
	for _, rest := range combinations(items[1:], k-1) {
		combo := append([]int{items[0]}, rest...)
		result = append(result, combo)
	}

	// skip first item
	result = append(result, combinations(items[1:], k)...)

	return result
}
