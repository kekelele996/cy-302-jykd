package service

import "math/rand"

// shuffle reorders a slice in place using the supplied random source.
func shuffle[T any](items []T, rng *rand.Rand) {
	rng.Shuffle(len(items), func(i, j int) {
		items[i], items[j] = items[j], items[i]
	})
}
