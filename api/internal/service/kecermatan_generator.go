package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	mathrand "math/rand/v2"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// ErrKecermatanPool: the deduplicated pool holds fewer than options_per_soal x item_length elements.
var ErrKecermatanPool = errors.New("kecermatan pool too small")

const seedBytes = 16

// KecermatanParams is the structure the generator needs; the service validates it before any generation.
type KecermatanParams struct {
	ItemLength     int
	OptionsPerSoal int
	SoalPerKolom   int
	KolomCount     int
}

// KecItem is one labelled reference item: item_length distinct elements.
type KecItem struct {
	Label    string                    `json:"label"`
	Elements []model.KecermatanElement `json:"elements"`
}

// KecSoal shows every reference item but one, in a shuffled order; Missing is the answer key.
type KecSoal struct {
	Shown   []string `json:"shown"`
	Missing string   `json:"missing"`
}

// KecColumn is one generated column: its reference set (fixed for all its soal) and its ordered soal.
type KecColumn struct {
	Reference []KecItem `json:"reference"`
	Soal      []KecSoal `json:"soal"`
}

// KecGeneration holds only the N/2 generated columns; kolom k and k+N/2 are the same content (FR-KEC-16/17).
type KecGeneration struct {
	KolomCount int
	Columns    []KecColumn
}

// Slot maps kolom k (1-based) to the generated column it shows and that column's 1-based index.
func (g KecGeneration) Slot(kolom int) (*KecColumn, int) {
	if kolom < 1 || kolom > g.KolomCount || len(g.Columns) == 0 {
		return nil, 0
	}
	source := (kolom-1)%len(g.Columns) + 1
	return &g.Columns[source-1], source
}

// AnswerKeys returns one row per kolom (N rows) of the missing label per soal. The second half repeats the
// first, but every kolom owns its row, because a student answers each kolom separately (FR-KEC-17).
func (g KecGeneration) AnswerKeys() [][]string {
	keys := make([][]string, 0, g.KolomCount)
	for kolom := 1; kolom <= g.KolomCount; kolom++ {
		column, _ := g.Slot(kolom)
		row := make([]string, len(column.Soal))
		for i, soal := range column.Soal {
			row[i] = soal.Missing
		}
		keys = append(keys, row)
	}
	return keys
}

func newKecermatanSeed() (string, error) {
	raw := make([]byte, seedBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("draw kecermatan seed: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

func validKecermatanSeed(seed string) bool {
	raw, err := hex.DecodeString(seed)
	return err == nil && len(raw) == seedBytes
}

// GenerateKecermatan builds the N/2 columns from the seed. It is a pure function of (seed, pool order,
// params): there is no clock and no global RNG, so the same inputs always give the same content.
func GenerateKecermatan(seed string, pool []model.KecermatanElement, p KecermatanParams) (KecGeneration, error) {
	half := p.KolomCount / 2
	gen := KecGeneration{KolomCount: p.KolomCount, Columns: make([]KecColumn, 0, half)}
	for source := 1; source <= half; source++ {
		column, err := GenerateKecermatanColumn(seed, pool, p, source)
		if err != nil {
			return KecGeneration{}, err
		}
		gen.Columns = append(gen.Columns, column)
	}
	return gen, nil
}

// GenerateKecermatanColumn builds generated column source (1-based, at most N/2) without the others.
func GenerateKecermatanColumn(seed string, pool []model.KecermatanElement, p KecermatanParams, source int) (KecColumn, error) {
	raw, err := hex.DecodeString(seed)
	if err != nil || len(raw) != seedBytes {
		return KecColumn{}, errors.New("kecermatan seed must be 16 bytes of hex")
	}
	if source < 1 || source > p.KolomCount/2 {
		return KecColumn{}, fmt.Errorf("kecermatan column %d is outside the %d generated columns", source, p.KolomCount/2)
	}
	need := p.OptionsPerSoal * p.ItemLength
	if need < 1 || p.OptionsPerSoal > len(kecermatanLabels) || len(pool) < need {
		return KecColumn{}, ErrKecermatanPool
	}
	rng := newColumnStream(raw, source)

	order := make([]int, len(pool))
	for i := range order {
		order[i] = i
	}
	// A partial Fisher-Yates: the first `need` positions are a uniform sample without replacement, so no
	// element can sit in two reference items or twice in one (FR-ADM-KEC-09).
	for i := range need {
		j := i + rng.below(len(order)-i)
		order[i], order[j] = order[j], order[i]
	}
	column := KecColumn{Reference: make([]KecItem, 0, p.OptionsPerSoal), Soal: make([]KecSoal, 0, p.SoalPerKolom)}
	labels := make([]string, 0, p.OptionsPerSoal)
	for item := range p.OptionsPerSoal {
		ref := KecItem{Label: kecermatanLabels[item], Elements: make([]model.KecermatanElement, 0, p.ItemLength)}
		for _, at := range order[item*p.ItemLength : (item+1)*p.ItemLength] {
			ref.Elements = append(ref.Elements, pool[at])
		}
		column.Reference = append(column.Reference, ref)
		labels = append(labels, ref.Label)
	}
	for range p.SoalPerKolom {
		missing := rng.below(len(labels))
		shown := make([]string, 0, len(labels)-1)
		for at, label := range labels {
			if at != missing {
				shown = append(shown, label)
			}
		}
		for i := len(shown) - 1; i > 0; i-- {
			j := rng.below(i + 1)
			shown[i], shown[j] = shown[j], shown[i]
		}
		column.Soal = append(column.Soal, KecSoal{Shown: shown, Missing: labels[missing]})
	}
	return column, nil
}

var kecermatanLabels = []string{"A", "B", "C", "D", "E", "F", "G", "H"}

// stream is a PCG seeded from sha256(seed, column). Only Uint64 is used from the standard generator, and
// below is local, so a future change to the standard library's bounded-int algorithms cannot change content.
type stream struct{ r *mathrand.Rand }

func newColumnStream(seed []byte, source int) stream {
	h := sha256.New()
	h.Write(seed)
	var col [4]byte
	binary.BigEndian.PutUint32(col[:], uint32(source))
	h.Write(col[:])
	sum := h.Sum(nil)
	return stream{r: mathrand.New(mathrand.NewPCG(binary.BigEndian.Uint64(sum[:8]), binary.BigEndian.Uint64(sum[8:16])))}
}

// below draws uniformly from [0, n) by rejection, so there is no modulo bias.
func (s stream) below(n int) int {
	bound := uint64(n)
	limit := ^uint64(0) - (^uint64(0))%bound
	for {
		if v := s.r.Uint64(); v < limit {
			return int(v % bound)
		}
	}
}

// kecermatanIdentity is what a student sees: text kinds compare by character, so a symbol "A" and the
// char A are one element; the others compare by their own key (emoji/shape id, image content hash).
func kecermatanIdentity(e model.KecermatanElement) string {
	switch e.Kind {
	case model.KecChar, model.KecNumber, model.KecSymbol:
		return "t:" + e.Value
	}
	return e.Kind + ":" + e.Key
}

// dedupeKecermatanPool keeps the first of every identity, preserving order so the pool stays deterministic.
func dedupeKecermatanPool(pool []model.KecermatanElement) []model.KecermatanElement {
	seen := make(map[string]bool, len(pool))
	out := make([]model.KecermatanElement, 0, len(pool))
	for _, e := range pool {
		id := kecermatanIdentity(e)
		if !seen[id] {
			seen[id] = true
			out = append(out, e)
		}
	}
	return out
}

func kecermatanChars() []model.KecermatanElement {
	out := make([]model.KecermatanElement, 0, 26)
	for c := 'A'; c <= 'Z'; c++ {
		out = append(out, model.KecermatanElement{Kind: model.KecChar, Key: string(c), Value: string(c)})
	}
	return out
}

func kecermatanNumbers() []model.KecermatanElement {
	out := make([]model.KecermatanElement, 0, 10)
	for c := '0'; c <= '9'; c++ {
		out = append(out, model.KecermatanElement{Kind: model.KecNumber, Key: string(c), Value: string(c)})
	}
	return out
}

func kecermatanShapes() []model.KecermatanElement {
	out := make([]model.KecermatanElement, 0, defaultShapeCount)
	for _, s := range shapeCatalog[:defaultShapeCount] {
		out = append(out, model.KecermatanElement{Kind: model.KecShape, Key: s.ID, Value: s.ID})
	}
	return out
}
