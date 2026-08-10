package application

// restrictOwnerIDs applies the dashboard access scope to an owner filter. A
// restricted scope with no allowed employees is deliberately represented by
// the impossible owner id 0 so repositories return no rows.
func restrictOwnerIDs(requested, allowed []int64, restricted bool) []int64 {
	if !restricted {
		return append([]int64(nil), requested...)
	}
	if len(allowed) == 0 {
		return []int64{0}
	}
	set := make(map[int64]struct{}, len(allowed))
	for _, id := range allowed {
		if id > 0 {
			set[id] = struct{}{}
		}
	}
	if len(requested) == 0 {
		result := make([]int64, 0, len(set))
		for id := range set {
			result = append(result, id)
		}
		return result
	}
	result := make([]int64, 0, len(requested))
	for _, id := range requested {
		if _, ok := set[id]; ok {
			result = append(result, id)
		}
	}
	if len(result) == 0 {
		return []int64{0}
	}
	return result
}

func restrictOwnerID(requested *int64, allowed []int64, restricted bool) *int64 {
	if !restricted {
		return requested
	}
	if requested == nil {
		if len(allowed) == 0 {
			zero := int64(0)
			return &zero
		}
		return nil
	}
	ids := restrictOwnerIDs(func() []int64 {
		if requested == nil {
			return nil
		}
		return []int64{*requested}
	}(), allowed, true)
	return &ids[0]
}

// RestrictOwnerIDForHTTP is the typed bridge used by handlers that expose a
// single owner filter (for example opportunities).
func RestrictOwnerIDForHTTP(requested *int64, allowed []int64, restricted bool) *int64 {
	return restrictOwnerID(requested, allowed, restricted)
}
