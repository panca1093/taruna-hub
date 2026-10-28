package service

import (
	"math"
	"strings"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

type partScore struct {
	Earned  float64
	Max     float64
	Correct int
}

// scorePart sums D04's points over one subtest. An unanswered question is simply not correct; its maximum still counts.
func scorePart(questions []model.AttemptQuestion, answers map[uuid.UUID][]string, w model.Weights) partScore {
	var out partScore
	for _, q := range questions {
		earned, maximum, correct := scoreQuestion(q, answers[q.QuestionID], w)
		out.Earned += earned
		out.Max += maximum
		if correct {
			out.Correct++
		}
	}
	return out
}

// scoreQuestion is the D04 rule for one question: the difficulty weight when the answer is fully correct, and for an
// option-weighted pg question the selected option's points (unset points count as 1). pg_multi is all-or-nothing.
func scoreQuestion(q model.AttemptQuestion, given []string, w model.Weights) (earned, maximum float64, correct bool) {
	if q.OptionWeighted && q.Type == model.QuestionPG {
		for _, o := range q.Options {
			if p := optionPoints(o); p > maximum {
				maximum = p
			}
		}
		if len(given) != 1 {
			return 0, maximum, false
		}
		for _, o := range q.Options {
			if o.ID == given[0] {
				earned = optionPoints(o)
				return earned, maximum, earned == maximum
			}
		}
		return 0, maximum, false
	}

	maximum = w.Of(q.Difficulty)
	if answeredCorrectly(q, given) {
		return maximum, maximum, true
	}
	return 0, maximum, false
}

func optionPoints(o model.QuestionOption) float64 {
	if o.Points == nil {
		return 1
	}
	return *o.Points
}

func answeredCorrectly(q model.AttemptQuestion, given []string) bool {
	if len(given) == 0 {
		return false
	}
	switch q.Type {
	case model.QuestionIsian:
		got := normalizeIsian(given[0], q.CaseSensitive)
		for _, accepted := range q.Correct {
			if got == normalizeIsian(accepted, q.CaseSensitive) {
				return true
			}
		}
		return false
	case model.QuestionPG, model.QuestionTrueFalse:
		return len(given) == 1 && len(q.Correct) == 1 && given[0] == q.Correct[0]
	case model.QuestionPGMulti:
		return len(q.Correct) > 0 && sameAnswer(given, q.Correct)
	}
	return false
}

// normalizeIsian trims, collapses internal whitespace and, unless the question opts into case sensitivity, ignores case (D04).
func normalizeIsian(s string, caseSensitive bool) string {
	s = strings.Join(strings.Fields(s), " ")
	if !caseSensitive {
		s = strings.ToLower(s)
	}
	return s
}

// percent is (earned / max) * 100 rounded to 2 decimals. Dividing before scaling keeps exact halves exact, so
// 1/8 is 12.5 and 1/32 rounds half up to 3.13 instead of drifting on a float product.
func percent(earned, maximum float64) float64 {
	if maximum <= 0 {
		return 0
	}
	p := math.Round(earned*10000/maximum) / 100
	return math.Min(100, math.Max(0, p))
}

func round4(x float64) float64 { return math.Round(x*10000) / 10000 }

func round2(x float64) float64 { return math.Round(x*100) / 100 }
