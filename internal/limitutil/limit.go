package limitutil

func EffectiveLimit(total, limit int) int {
	if limit <= 0 || total < limit {
		return total
	}
	return limit
}

func CountOrLimit(total, limit int) (string, int) {
	if limit > 0 && total >= limit {
		return "limit", limit
	}
	return "count", total
}
