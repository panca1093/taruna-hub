package service

import (
	"errors"
	"fmt"
	"math"
)

// KecermatanScoringVersion is the formula new attempts are scored with. Version 2 is the client's revised formula;
// version 1, the baseline the owner approved on 2026-10-28, stays readable so stored results can be recomputed
// from raw counts.
const KecermatanScoringVersion = 2

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

const (
	kecV2KetahananBase      = 72.0
	kecV2KetahananSlope     = 1.6
	kecV2StabilityBase      = 80.0
	kecV2StabilitySDFactor  = 1.5
	kecV2KetelitianScale    = 81.0
	kecV2KecepatanCap       = 74.0
	kecV2KecepatanSpan      = 44.0
	kecV2KecepatanFloor     = 30.0
	kecV2KecepatanMid       = 60.0
	kecV2KecepatanTop       = 80.0
	kecV2KecepatanMidSpan   = 20.0
	kecV2KecepatanLowFactor = 2.0
	kecV2WeightKecepatan    = 0.3
	kecV2WeightKetahanan    = 0.3
	kecV2WeightKetelitian   = 0.4
	kecV2WeightA            = 0.6
	kecV2WeightB            = 0.4
)

// ScoreKecermatan is the one scoring function: a pure function of the per-kolom counts and the version.
// Version 2 (client revision), p = dikerjakan / all soal * 100:
//
//	Kecepatan  = 80 if p >= 74; 60 + (p-30)*20/44 if 30 <= p < 74; 2p if p < 30
//	Ketahanan  = 0.6A + 0.4B, A = 72 + (avg Benar of kolom N/2+1..N - avg Benar of kolom 1..N/2) * 1.6,
//	             B = 80 - 1.5 * sample SD (n-1) of dikerjakan per kolom
//	Ketelitian = (1 - Salah / dikerjakan) * 81
//
// NilaiAkhir = 0.3 Kecepatan + 0.3 Ketahanan + 0.4 Ketelitian, with the same zero-denominator, clamp and rounding rules.
//
// Version 1:
//
//	Kecepatan  = ((Benar + Salah) / all soal - 0.05) * 100
//	Ketahanan  = (avg Benar of kolom N/2+1..N / avg Benar of kolom 1..N/2 - 0.05) * 100
//	Ketelitian = Benar / (Benar + Salah) * 100
//
// A zero denominator makes that score 0. Each is clamped to 0-100 and rounded to two decimals, and
// NilaiAkhir = 0.3 Kecepatan + 0.45 Ketelitian + 0.25 Ketahanan over those clamped, rounded values, rounded again.
func ScoreKecermatan(version int, kolom []KecermatanKolomCount) (KecermatanScores, error) {
	if version != 1 && version != 2 {
		return KecermatanScores{}, fmt.Errorf("%w: %d", ErrKecermatanScoringVersion, version)
	}
	if len(kolom) < 2 || len(kolom)%2 != 0 {
		return KecermatanScores{}, fmt.Errorf("kecermatan scoring needs an even number of kolom, got %d", len(kolom))
	}
	if version == 2 {
		return scoreV2(kolom), nil
	}
	return scoreV1(kolom), nil
}

func scoreV1(kolom []KecermatanKolomCount) KecermatanScores {
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
		Version:       1,
		Kecepatan:     clampedScore(raw.KecepatanRaw),
		Ketahanan:     clampedScore(raw.KetahananRaw),
		Ketelitian:    clampedScore(raw.KetelitianRaw),
		KecepatanRaw:  raw.KecepatanRaw,
		KetahananRaw:  raw.KetahananRaw,
		KetelitianRaw: raw.KetelitianRaw,
	}
	out.NilaiAkhir = roundScore2(0.3*out.Kecepatan + 0.45*out.Ketelitian + 0.25*out.Ketahanan)
	return out
}

func scoreV2(kolom []KecermatanKolomCount) KecermatanScores {
	var benar, salah, total int
	for _, k := range kolom {
		benar += k.Benar
		salah += k.Salah
		total += k.Benar + k.Salah + k.Kosong
	}
	dikerjakan := benar + salah

	var raw KecermatanScores
	if total > 0 {
		p := float64(dikerjakan) * 100 / float64(total)
		switch {
		case p >= kecV2KecepatanCap:
			raw.KecepatanRaw = kecV2KecepatanTop
		case p >= kecV2KecepatanFloor:
			raw.KecepatanRaw = kecV2KecepatanMid + (p-kecV2KecepatanFloor)*kecV2KecepatanMidSpan/kecV2KecepatanSpan
		default:
			// The client writes this branch as 2 x mentah x 100 with mentah as a fraction, which is 2*p.
			raw.KecepatanRaw = kecV2KecepatanLowFactor * p
		}
	}
	a, _, b := kecermatanV2Stability(kolom)
	raw.KetahananRaw = kecV2WeightA*a + kecV2WeightB*b
	if dikerjakan > 0 {
		raw.KetelitianRaw = (1 - float64(salah)/float64(dikerjakan)) * kecV2KetelitianScale
	}

	out := KecermatanScores{
		Version:       2,
		Kecepatan:     clampedScore(raw.KecepatanRaw),
		Ketahanan:     clampedScore(raw.KetahananRaw),
		Ketelitian:    clampedScore(raw.KetelitianRaw),
		KecepatanRaw:  raw.KecepatanRaw,
		KetahananRaw:  raw.KetahananRaw,
		KetelitianRaw: raw.KetelitianRaw,
	}
	out.NilaiAkhir = roundScore2(kecV2WeightKecepatan*out.Kecepatan + kecV2WeightKetahanan*out.Ketahanan + kecV2WeightKetelitian*out.Ketelitian)
	return out
}

// kecermatanV2Stability returns the unrounded A, sample SD and B of the v2 Ketahanan; kolom is already validated even and >= 2.
func kecermatanV2Stability(kolom []KecermatanKolomCount) (a, sd, b float64) {
	n := len(kolom)
	half := n / 2
	var firstHalf, secondHalf, sum float64
	for i, k := range kolom {
		if i < half {
			firstHalf += float64(k.Benar)
		} else {
			secondHalf += float64(k.Benar)
		}
		sum += float64(k.Benar + k.Salah)
	}
	a = kecV2KetahananBase + (secondHalf/float64(half)-firstHalf/float64(half))*kecV2KetahananSlope
	mean := sum / float64(n)
	var ss float64
	for _, k := range kolom {
		d := float64(k.Benar+k.Salah) - mean
		ss += d * d
	}
	sd = math.Sqrt(ss / float64(n-1))
	b = kecV2StabilityBase - kecV2StabilitySDFactor*sd
	return a, sd, b
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
