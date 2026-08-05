package reporting

import "sort"

type CustomerEvent struct {
	ID         string
	Stage      string
	OccurredAt int64
}

func AggregateCustomers(events []CustomerEvent) ReportResult {
	seen := map[string]bool{}
	counts := map[string]*float64{}
	for _, e := range events {
		if seen[e.ID] {
			continue
		}
		seen[e.ID] = true
		v := counts[e.Stage]
		if v == nil {
			x := float64(0)
			v = &x
			counts[e.Stage] = v
		}
		*v++
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	dims := make([]Dimension, 0, len(keys))
	for _, k := range keys {
		dims = append(dims, Dimension{Key: k, Label: k, Value: *counts[k]})
	}
	return ReportResult{Summary: counts, Dimensions: dims, Items: []map[string]any{}}
}
