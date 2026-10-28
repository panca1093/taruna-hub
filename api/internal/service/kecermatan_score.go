package service

import (
	"errors"
	"fmt"
	"math"
)

// KecermatanScoringVersion is the formula new attempts are scored with. Version 1 is the provisional baseline
// the owner approved on 2026-10-28; the client will revise it (PRD section 10, question 1, D03), and a revised
// formula becomes version 2 while version 1 stays readable, so stored results can be recomputed from raw counts.
const KecermatanScoringVersion = 1

var ErrKecermatanScoringVersion = errors.New("unknown kecermatan scoring version")

// KecermatanKolomCount is the authoritative outcome of one kolom.
type KecermatanKolomCount struct {
	Benar  int
	Salah  int
	Kosong int
}

// KecermatanScores holds the composite scores under one version. The four plain fields are clamped to 0-100 and
// rounded to two decimals; the Raw fields are the same values before clamping and rounding.
type KecermatanScores struct {
	Version       int
	Kecepatan     float64
	Ketahanan     float64
	Ketelitian    float64
	NilaiAkhir    float64
	KecepatanRaw  float64
	KetahananRaw  float64
	KetelitianRaw float64
}

// ScoreKecermatan is the one scoring function: a pure function of the per-kolom counts and the version.
// Version 1:
//
//	Kecepatan  = ((Benar + Salah) / all soal - 0.05) * 100
//	Ketahanan  = (avg Benar of kolom N/2+1..N / avg Benar of kolom 1..N/2 - 0.05) * 100
//	Ketelitian = Benar / (Benar + Salah) * 100
//
// A zero denominator makes that score 0. Each is clamped to 0-100 and rounded to two decimals, and
// NilaiAkhir = 0.3 Kecepatan + 0.45 Ketelitian + 0.25 Ketahanan over those clamped, rounded values, rounded again.
func ScoreKecermatan(version int, kolom []KecermatanKolomCount) (KecermatanScores, error) {
	if version != 1 {
		return KecermatanScores{}, fmt.Errorf("%w: %d", ErrKecermatanScoringVersion, version)
	}
	if len(kolom) < 2 || len(kolom)%2 != 0 {
		return KecermatanScores{}, fmt.Errorf("kecermatan scoring needs an even number of kolom, got %d", len(kolom))
	}
	var benar, salah, total, firstHalf, secondHalf int
	half := len(kolom) / 2
	for i, k := range kolom {
		benar += k.Benar
		salah += k.Salah
		total += k.Benar + k.Salah + k.Kosong
		if i < half {
			firstHalf += k.Benar
		} else {
			secondHalf += k.Benar
		}
	}
	dikerjakan := benar + salah

	// Halves have the same kolom count, so the ratio of averages is the ratio of sums.
	var raw KecermatanScores
	if total > 0 {
		raw.KecepatanRaw = (float64(dikerjakan)/float64(total) - 0.05) * 100
	}
	if firstHalf > 0 {
		raw.KetahananRaw = (float64(secondHalf)/float64(firstHalf) - 0.05) * 100
	}
	if dikerjakan > 0 {
		raw.KetelitianRaw = float64(benar) / float64(dikerjakan) * 100
	}

	out := KecermatanScores{
		Version:       version,
		Kecepatan:     clampedScore(raw.KecepatanRaw),
		Ketahanan:     clampedScore(raw.KetahananRaw),
		Ketelitian:    clampedScore(raw.KetelitianRaw),
		KecepatanRaw:  raw.KecepatanRaw,
		KetahananRaw:  raw.KetahananRaw,
		KetelitianRaw: raw.KetelitianRaw,
	}
	out.NilaiAkhir = roundScore2(0.3*out.Kecepatan + 0.45*out.Ketelitian + 0.25*out.Ketahanan)
	return out, nil
}

func clampedScore(v float64) float64 {
	return roundScore2(math.Min(100, math.Max(0, v)))
}

// roundScore2 rounds half away from zero to two decimals and never returns negative zero.
func roundScore2(v float64) float64 {
	r := math.Round(v*100) / 100
	if r == 0 {
		return 0
	}
	return r
}
