package service

import (
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

func testSeed(t testing.TB) string {
	t.Helper()
	seed, err := newKecermatanSeed()
	if err != nil {
		t.Fatal(err)
	}
	return seed
}

func symbolPool(n int) []model.KecermatanElement {
	pool := make([]model.KecermatanElement, 0, n)
	for i := range n {
		v := string(rune('a' + i%26))
		if i >= 26 {
			v = string(rune(0x4e00 + i))
		}
		pool = append(pool, model.KecermatanElement{Kind: model.KecSymbol, Key: v, Value: v})
	}
	return pool
}

func identityOf(e model.KecermatanElement) string { return kecermatanIdentity(e) }

func TestKecermatanSeedIs128BitHex(t *testing.T) {
	a, b := testSeed(t), testSeed(t)
	raw, err := hex.DecodeString(a)
	if err != nil || len(raw) != 16 {
		t.Fatalf("seed %q is not 16 bytes of hex: %v", a, err)
	}
	if a == b {
		t.Error("two seeds are equal")
	}
}

func TestKecermatanColumnNeverRepeatsAnElement(t *testing.T) {
	params := KecermatanParams{ItemLength: 4, OptionsPerSoal: 5, SoalPerKolom: 50, KolomCount: 10}
	// The exact-fit pool leaves the generator no slack: every element is used by some reference item.
	exact := params.ItemLength * params.OptionsPerSoal
	pools := map[string][]model.KecermatanElement{
		"exact fit": symbolPool(exact),
		"roomy":     symbolPool(exact + 17),
		"mix across sources": dedupeKecermatanPool(append(append(kecermatanChars(), kecermatanNumbers()...),
			model.KecermatanElement{Kind: model.KecSymbol, Key: "A", Value: "A"})),
	}
	for name, pool := range pools {
		pool, name := pool, name
		t.Run(name, func(t *testing.T) {
			for run := range 300 {
				seed := testSeed(t)
				gen, err := GenerateKecermatan(seed, pool, params)
				if err != nil {
					t.Fatalf("run %d: %v", run, err)
				}
				for k, column := range gen.Columns {
					seen := map[string]string{}
					if len(column.Reference) != params.OptionsPerSoal {
						t.Fatalf("seed %s column %d: %d reference items, want %d", seed, k+1, len(column.Reference), params.OptionsPerSoal)
					}
					for _, item := range column.Reference {
						if len(item.Elements) != params.ItemLength {
							t.Fatalf("seed %s column %d item %s: %d elements, want %d", seed, k+1, item.Label, len(item.Elements), params.ItemLength)
						}
						for _, e := range item.Elements {
							id := identityOf(e)
							if prev, dup := seen[id]; dup {
								t.Fatalf("seed %s column %d: element %q is in item %s and again in %s", seed, k+1, id, prev, item.Label)
							}
							seen[id] = item.Label
						}
					}
				}
			}
		})
	}
}

func TestKecermatanSoalShowsAllButOneReferenceItem(t *testing.T) {
	params := KecermatanParams{ItemLength: 2, OptionsPerSoal: 6, SoalPerKolom: 100, KolomCount: 50}
	gen, err := GenerateKecermatan(testSeed(t), symbolPool(40), params)
	if err != nil {
		t.Fatal(err)
	}
	missingSeen := map[string]bool{}
	for _, column := range gen.Columns {
		if len(column.Soal) != params.SoalPerKolom {
			t.Fatalf("%d soal, want %d", len(column.Soal), params.SoalPerKolom)
		}
		labels := map[string]bool{}
		for _, item := range column.Reference {
			labels[item.Label] = true
		}
		for n, soal := range column.Soal {
			if !labels[soal.Missing] {
				t.Fatalf("soal %d: missing %q is not a reference label", n+1, soal.Missing)
			}
			if len(soal.Shown) != params.OptionsPerSoal-1 {
				t.Fatalf("soal %d shows %d items, want %d", n+1, len(soal.Shown), params.OptionsPerSoal-1)
			}
			shown := map[string]bool{}
			for _, label := range soal.Shown {
				if !labels[label] || shown[label] || label == soal.Missing {
					t.Fatalf("soal %d: shown %v with missing %q is not a distinct subset of the reference", n+1, soal.Shown, soal.Missing)
				}
				shown[label] = true
			}
			missingSeen[soal.Missing] = true
		}
	}
	if len(missingSeen) != params.OptionsPerSoal {
		t.Errorf("only %d of %d reference items were ever the missing one", len(missingSeen), params.OptionsPerSoal)
	}
}

func TestKecermatanSecondHalfReplaysTheFirstExactly(t *testing.T) {
	params := KecermatanParams{ItemLength: 3, OptionsPerSoal: 5, SoalPerKolom: 50, KolomCount: 12}
	gen, err := GenerateKecermatan(testSeed(t), symbolPool(30), params)
	if err != nil {
		t.Fatal(err)
	}
	if len(gen.Columns) != params.KolomCount/2 {
		t.Fatalf("generated %d columns, want only N/2 = %d", len(gen.Columns), params.KolomCount/2)
	}
	keys := gen.AnswerKeys()
	if len(keys) != params.KolomCount {
		t.Fatalf("%d answer-key rows, want one per kolom (%d)", len(keys), params.KolomCount)
	}
	half := params.KolomCount / 2
	for k := 1; k <= half; k++ {
		first, source := gen.Slot(k)
		again, againSource := gen.Slot(k + half)
		if source != k || againSource != k {
			t.Errorf("kolom %d and %d come from sources %d and %d, want both %d", k, k+half, source, againSource, k)
		}
		if !reflect.DeepEqual(first, again) {
			t.Errorf("kolom %d and %d differ: references, order, subsets and missing item must be identical", k, k+half)
		}
		if !reflect.DeepEqual(keys[k-1], keys[k-1+half]) {
			t.Errorf("answer keys of kolom %d and %d differ", k, k+half)
		}
		if len(keys[k-1]) != params.SoalPerKolom {
			t.Errorf("kolom %d key has %d answers, want %d", k, len(keys[k-1]), params.SoalPerKolom)
		}
	}
	// Separate identity: mutating one kolom's key row must not leak into its replay.
	keys[0][0] = "?"
	if again := gen.AnswerKeys(); again[0][0] == "?" || again[half][0] == "?" {
		t.Error("answer keys share backing storage between kolom or calls")
	}
	if first, _ := gen.Slot(1); first.Soal[0].Missing == "?" {
		t.Error("editing a key row changed the generated content")
	}
	if c, source := gen.Slot(half + half + 1); c != nil || source != 0 {
		t.Errorf("Slot past kolom_count = %v, %d; want nothing", c, source)
	}
}

func TestKecermatanGeneratorIsDeterministicInItsSeed(t *testing.T) {
	params := KecermatanParams{ItemLength: 3, OptionsPerSoal: 4, SoalPerKolom: 50, KolomCount: 10}
	pool := symbolPool(60)
	seed := testSeed(t)
	a, err := GenerateKecermatan(seed, pool, params)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := GenerateKecermatan(seed, pool, params)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("the same seed produced two different generations")
	}
	other, _ := GenerateKecermatan(testSeed(t), pool, params)
	if reflect.DeepEqual(a, other) {
		t.Error("two different seeds produced the same generation")
	}
	for source := 1; source <= params.KolomCount/2; source++ {
		column, err := GenerateKecermatanColumn(seed, pool, params, source)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(column, a.Columns[source-1]) {
			t.Errorf("column %d generated alone differs from the same column of the full generation", source)
		}
	}
	// A frozen vector pins the algorithm: changing it would silently change every stored test's content.
	frozen, err := GenerateKecermatanColumn("00112233445566778899aabbccddeeff", symbolPool(12),
		KecermatanParams{ItemLength: 2, OptionsPerSoal: 3, SoalPerKolom: 50, KolomCount: 10}, 1)
	if err != nil {
		t.Fatal(err)
	}
	var got string
	for _, item := range frozen.Reference {
		for _, e := range item.Elements {
			got += e.Value
		}
		got += "|"
	}
	got += fmt.Sprint(frozen.Soal[0].Shown, frozen.Soal[0].Missing)
	if got != frozenKecermatanColumn {
		t.Errorf("frozen vector = %q, want %q", got, frozenKecermatanColumn)
	}
}

const frozenKecermatanColumn = "gb|ah|fl|[A C]B"

func TestKecermatanGeneratorRefusesAPoolThatIsTooSmall(t *testing.T) {
	params := KecermatanParams{ItemLength: 4, OptionsPerSoal: 5, SoalPerKolom: 50, KolomCount: 10}
	_, err := GenerateKecermatan(testSeed(t), symbolPool(19), params)
	if !errors.Is(err, ErrKecermatanPool) {
		t.Errorf("19 elements for 20 needed: err = %v, want ErrKecermatanPool", err)
	}
	if _, err := GenerateKecermatan("not-a-seed", symbolPool(40), params); err == nil {
		t.Error("a malformed seed was accepted")
	}
	if _, err := GenerateKecermatanColumn(testSeed(t), symbolPool(40), params, 6); err == nil {
		t.Error("source column 6 of a 10-kolom test (5 generated) was accepted")
	}
}

func TestKecermatanGeneratorStaysBoundedAtTheLargestTest(t *testing.T) {
	params := KecermatanParams{ItemLength: 5, OptionsPerSoal: 8, SoalPerKolom: 100, KolomCount: 50}
	pool := symbolPool(500)
	start := time.Now()
	gen, err := GenerateKecermatan(testSeed(t), pool, params)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if len(gen.Columns) != 25 || len(gen.AnswerKeys()) != 50 {
		t.Fatalf("%d columns, %d key rows, want 25 and 50", len(gen.Columns), len(gen.AnswerKeys()))
	}
	if elapsed > time.Second {
		t.Errorf("50 kolom x 100 soal took %s, want well under a second", elapsed)
	}
}

func TestKecermatanBuiltinPools(t *testing.T) {
	if n := len(kecermatanChars()); n != 26 {
		t.Errorf("char pool = %d, want A-Z", n)
	}
	if n := len(kecermatanNumbers()); n != 10 {
		t.Errorf("number pool = %d, want 0-9", n)
	}
	if n := len(kecermatanShapes()); n != 7 {
		t.Errorf("shape pool = %d, want the 7 built-in shapes", n)
	}
}

func TestKecermatanPoolDedupesWhatLooksTheSame(t *testing.T) {
	// A symbol "A" is the same thing on screen as the char A; counting both would let a column show it twice.
	pool := dedupeKecermatanPool(append(kecermatanChars(),
		model.KecermatanElement{Kind: model.KecSymbol, Key: "A", Value: "A"},
		model.KecermatanElement{Kind: model.KecSymbol, Key: "★", Value: "★"},
		model.KecermatanElement{Kind: model.KecSymbol, Key: "★", Value: "★"}))
	if len(pool) != 27 {
		t.Errorf("pool of 26 letters + symbol A + two stars = %d, want 27", len(pool))
	}
}
