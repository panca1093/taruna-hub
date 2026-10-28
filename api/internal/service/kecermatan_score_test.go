package service

import (
	"errors"
	"math"
	"testing"
)

func uniformKolom(n int, c KecermatanKolomCount) []KecermatanKolomCount {
	out := make([]KecermatanKolomCount, n)
	for i := range out {
		out[i] = c
	}
	return out
}

// halves builds N kolom whose first N/2 and last N/2 share one count each.
func halves(n int, first, second KecermatanKolomCount) []KecermatanKolomCount {
	out := make([]KecermatanKolomCount, 0, n)
	out = append(out, uniformKolom(n/2, first)...)
	return append(out, uniformKolom(n/2, second)...)
}

func TestScoreKecermatanVersion1(t *testing.T) {
	cases := []struct {
		name  string
		kolom []KecermatanKolomCount
		want  [4]float64 // kecepatan, ketahanan, ketelitian, nilai akhir
		raw   [3]float64 // kecepatan, ketahanan, ketelitian before clamping
	}{
		{
			// N=10: dikerjakan 425 of 500, last 5 kolom average 36 correct against 40 in the first 5.
			name:  "N=10 worked example",
			kolom: halves(10, KecermatanKolomCount{40, 5, 5}, KecermatanKolomCount{36, 4, 10}),
			want:  [4]float64{80, 85, 89.41, 85.48},
			raw:   [3]float64{80, 85, 89.4118},
		},
		{
			// N=12 compares the last 6 kolom with the first 6; a ratio of 1.2 overflows ketahanan to 115.
			name:  "N>10 with ketahanan overflow",
			kolom: halves(12, KecermatanKolomCount{30, 10, 10}, KecermatanKolomCount{36, 4, 10}),
			want:  [4]float64{75, 100, 82.5, 84.63},
			raw:   [3]float64{75, 115, 82.5},
		},
		{
			name:  "zero attempts: every denominator is zero or the speed is below the floor",
			kolom: uniformKolom(10, KecermatanKolomCount{0, 0, 50}),
			want:  [4]float64{0, 0, 0, 0},
			raw:   [3]float64{-5, 0, 0},
		},
		{
			name:  "zero correct in the first half makes ketahanan 0 whatever the second half does",
			kolom: halves(10, KecermatanKolomCount{0, 20, 30}, KecermatanKolomCount{25, 5, 20}),
			want:  [4]float64{45, 0, 50, 36},
			raw:   [3]float64{45, 0, 50},
		},
		{
			name:  "a tiny first half against a full second half clamps to 100",
			kolom: halves(10, KecermatanKolomCount{1, 0, 49}, KecermatanKolomCount{50, 0, 0}),
			want:  [4]float64{46, 100, 100, 83.8},
			raw:   [3]float64{46, 4995, 100},
		},
		{
			// Rounded components feed the final: 0.3*36 + 0.45*41.46 + 0.25*65 = 45.707.
			name:  "rounding to two decimals",
			kolom: halves(10, KecermatanKolomCount{10, 10, 30}, KecermatanKolomCount{7, 14, 29}),
			want:  [4]float64{36, 65, 41.46, 45.71},
			raw:   [3]float64{36, 65, 41.4634},
		},
		{
			name:  "everything correct",
			kolom: uniformKolom(10, KecermatanKolomCount{50, 0, 0}),
			want:  [4]float64{95, 95, 100, 97.25},
			raw:   [3]float64{95, 95, 100},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ScoreKecermatan(1, c.kolom)
			if err != nil {
				t.Fatalf("ScoreKecermatan: %v", err)
			}
			if got.Version != 1 {
				t.Errorf("version = %d, want 1", got.Version)
			}
			for _, pair := range []struct {
				name      string
				got, want float64
			}{
				{"kecepatan", got.Kecepatan, c.want[0]},
				{"ketahanan", got.Ketahanan, c.want[1]},
				{"ketelitian", got.Ketelitian, c.want[2]},
				{"nilai akhir", got.NilaiAkhir, c.want[3]},
			} {
				if math.Abs(pair.got-roundScore2(pair.want)) > 1e-9 {
					t.Errorf("%s = %v, want %v", pair.name, pair.got, roundScore2(pair.want))
				}
			}
			for _, pair := range []struct {
				name      string
				got, want float64
			}{
				{"kecepatan raw", got.KecepatanRaw, c.raw[0]},
				{"ketahanan raw", got.KetahananRaw, c.raw[1]},
				{"ketelitian raw", got.KetelitianRaw, c.raw[2]},
			} {
				if math.Abs(pair.got-pair.want) > 1e-3 {
					t.Errorf("%s = %v, want %v", pair.name, pair.got, pair.want)
				}
			}
		})
	}
}

func TestScoreKecermatanFinalUsesClampedScores(t *testing.T) {
	got, err := ScoreKecermatan(1, halves(12, KecermatanKolomCount{30, 10, 10}, KecermatanKolomCount{36, 4, 10}))
	if err != nil {
		t.Fatalf("ScoreKecermatan: %v", err)
	}
	// 115 clamps to 100 before weighting; weighting the raw 115 would give 87.13.
	want := roundScore2(0.3*got.Kecepatan + 0.45*got.Ketelitian + 0.25*got.Ketahanan)
	if got.NilaiAkhir != want || got.Ketahanan != 100 {
		t.Errorf("final = %v ketahanan = %v, want %v and 100", got.NilaiAkhir, got.Ketahanan, want)
	}
}

func TestScoreKecermatanStaysInRange(t *testing.T) {
	for benar := 0; benar <= 50; benar += 7 {
		for salah := 0; salah+benar <= 50; salah += 9 {
			got, err := ScoreKecermatan(1, halves(10, KecermatanKolomCount{benar, salah, 50 - benar - salah}, KecermatanKolomCount{50 - salah, salah, 0}))
			if err != nil {
				t.Fatalf("ScoreKecermatan: %v", err)
			}
			for _, v := range []float64{got.Kecepatan, got.Ketahanan, got.Ketelitian, got.NilaiAkhir} {
				if v < 0 || v > 100 || math.Signbit(v) {
					t.Fatalf("score %v outside 0-100 for benar=%d salah=%d: %+v", v, benar, salah, got)
				}
			}
		}
	}
}

func TestScoreKecermatanRejects(t *testing.T) {
	if _, err := ScoreKecermatan(99, uniformKolom(10, KecermatanKolomCount{1, 1, 1})); !errors.Is(err, ErrKecermatanScoringVersion) {
		t.Errorf("unknown version err = %v, want ErrKecermatanScoringVersion", err)
	}
	for _, n := range []int{0, 1, 9} {
		if _, err := ScoreKecermatan(1, uniformKolom(n, KecermatanKolomCount{1, 1, 1})); err == nil {
			t.Errorf("%d kolom accepted, want an error for an odd or empty set", n)
		}
	}
}

func TestScoreKecermatanIsDeterministic(t *testing.T) {
	kolom := halves(10, KecermatanKolomCount{10, 10, 30}, KecermatanKolomCount{7, 14, 29})
	a, _ := ScoreKecermatan(1, kolom)
	b, _ := ScoreKecermatan(1, kolom)
	if a != b {
		t.Errorf("same counts gave %+v then %+v", a, b)
	}
}

func TestRound2(t *testing.T) {
	for in, want := range map[float64]float64{
		41.4634146: 41.46, 0.125: 0.13, 99.995: 100, 0: 0, -0.0000001: 0, 84.625: 84.63, 12.344: 12.34,
	} {
		if got := roundScore2(in); got != want || math.Signbit(got) {
			t.Errorf("roundScore2(%v) = %v, want %v", in, got, want)
		}
	}
}
