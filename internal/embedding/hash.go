package embedding

import (
	"hash/fnv"
	"math"
	"regexp"
	"strings"
)

const HashVectorSize = 256

var tokenRE = regexp.MustCompile(`[a-z0-9_]+`)

type HashProvider struct {
	Size int
}

func NewHashProvider(size int) HashProvider {
	if size <= 0 {
		size = HashVectorSize
	}
	return HashProvider{Size: size}
}

func (p HashProvider) Embed(text string) []float32 {
	size := p.Size
	if size <= 0 {
		size = HashVectorSize
	}
	vec := make([]float32, size)
	tokens := tokenRE.FindAllString(strings.ToLower(text), -1)
	for _, token := range tokens {
		idx := hashToken(token) % uint64(size)
		vec[idx] += 1
	}
	normalize(vec)
	return vec
}

func hashToken(token string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(token))
	return h.Sum64()
}

func normalize(vec []float32) {
	var sum float64
	for _, v := range vec {
		sum += float64(v * v)
	}
	if sum == 0 {
		return
	}
	norm := float32(math.Sqrt(sum))
	for i := range vec {
		vec[i] /= norm
	}
}
