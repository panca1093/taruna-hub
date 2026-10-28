package service

import (
	"reflect"
	"sort"
	"testing"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

func pts(v float64) *float64 { return &v }

func scoredQuestion(typ, difficulty string, correct []string, options ...model.QuestionOption) model.AttemptQuestion {
	return model.AttemptQuestion{
		QuestionID: uuid.New(), Type: typ, Difficulty: difficulty, Correct: correct, Options: options,
	}
}

func opts(ids ...string) []model.QuestionOption {
	out := make([]model.QuestionOption, 0, len(ids))
	for _, id := range ids {
		out = append(out, model.QuestionOption{ID: id, Label: id})
	}
	return out
}

// Every expectation below is worked out by hand from D04 with the default weights 1/2/3.
func TestScoreQuestionByType(t *testing.T) {
	w := model.DefaultWeights

	pg := scoredQuestion(model.QuestionPG, model.DifficultyEasy, []string{"b"}, opts("a", "b", "c")...)
	multi := scoredQuestion(model.QuestionPGMulti, model.DifficultyMedium, []string{"a", "c"}, opts("a", "b", "c")...)
	isian := scoredQuestion(model.QuestionIsian, model.DifficultyHard, []string{"Jakarta Pusat", "DKI Jakarta"})
	strict := scoredQuestion(model.QuestionIsian, model.DifficultyHard, []string{"Jakarta"})
	strict.CaseSensitive = true
	trueFalse := scoredQuestion(model.QuestionTrueFalse, model.DifficultyEasy, []string{"salah"}, opts("benar", "salah")...)
	weighted := scoredQuestion(model.QuestionPG, model.DifficultyMedium, nil,
		model.QuestionOption{ID: "a", Label: "a", Points: pts(0.5)},
		model.QuestionOption{ID: "b", Label: "b", Points: pts(3)},
		model.QuestionOption{ID: "c", Label: "c"},
	)
	weighted.OptionWeighted = true

	cases := []struct {
		name    string
		q       model.AttemptQuestion
		given   []string
		earned  float64
		max     float64
		correct bool
	}{
		{"pg right earns the easy weight", pg, []string{"b"}, 1, 1, true},
		{"pg wrong earns nothing", pg, []string{"a"}, 0, 1, false},
		{"pg unanswered earns nothing", pg, nil, 0, 1, false},
		{"pg with two picks is wrong", pg, []string{"a", "b"}, 0, 1, false},
		{"pg_multi exact set in any order earns the medium weight", multi, []string{"c", "a"}, 2, 2, true},
		{"pg_multi missing one is all-or-nothing", multi, []string{"a"}, 0, 2, false},
		{"pg_multi with an extra is all-or-nothing", multi, []string{"a", "b", "c"}, 0, 2, false},
		{"isian trims, collapses spaces and ignores case", isian, []string{"  jakarta   PUSAT "}, 3, 3, true},
		{"isian takes any accepted answer", isian, []string{"dki jakarta"}, 3, 3, true},
		{"isian wrong", isian, []string{"Bandung"}, 0, 3, false},
		{"isian case sensitive question refuses a lower-cased answer", strict, []string{"jakarta"}, 0, 3, false},
		{"isian case sensitive question accepts the exact case", strict, []string{" Jakarta "}, 3, 3, true},
		{"benar_salah right", trueFalse, []string{"salah"}, 1, 1, true},
		{"benar_salah wrong", trueFalse, []string{"benar"}, 0, 1, false},
		{"option weighted scores the chosen option's points", weighted, []string{"a"}, 0.5, 3, false},
		{"option weighted with unset points counts 1", weighted, []string{"c"}, 1, 3, false},
		{"option weighted best option is the maximum", weighted, []string{"b"}, 3, 3, true},
		{"option weighted unanswered keeps its maximum", weighted, nil, 0, 3, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			earned, max, correct := scoreQuestion(tc.q, tc.given, w)
			if earned != tc.earned || max != tc.max || correct != tc.correct {
				t.Errorf("got earned=%v max=%v correct=%v, want %v %v %v", earned, max, correct, tc.earned, tc.max, tc.correct)
			}
		})
	}
}

func TestScorePartSumsAHandCalculatedSubtest(t *testing.T) {
	pg := scoredQuestion(model.QuestionPG, model.DifficultyEasy, []string{"b"}, opts("a", "b")...)
	multi := scoredQuestion(model.QuestionPGMulti, model.DifficultyMedium, []string{"a", "c"}, opts("a", "b", "c")...)
	isian := scoredQuestion(model.QuestionIsian, model.DifficultyHard, []string{"Jakarta"})
	weighted := scoredQuestion(model.QuestionPG, model.DifficultyEasy, nil,
		model.QuestionOption{ID: "a", Label: "a", Points: pts(0.5)}, model.QuestionOption{ID: "b", Label: "b", Points: pts(3)})
	weighted.OptionWeighted = true
	questions := []model.AttemptQuestion{pg, multi, isian, weighted}
	answers := map[uuid.UUID][]string{
		pg.QuestionID:       {"b"},        // 1 of 1
		multi.QuestionID:    {"a"},        // 0 of 2
		isian.QuestionID:    {" JAKARTA"}, // 3 of 3
		weighted.QuestionID: {"a"},        // 0.5 of 3
	}

	got := scorePart(questions, answers, model.DefaultWeights)
	if got.Earned != 4.5 || got.Max != 9 || got.Correct != 2 {
		t.Errorf("default weights: %+v, want earned 4.5 max 9 correct 2", got)
	}

	// Weights 2/4/6 double every difficulty point; option points never move.
	got = scorePart(questions, answers, model.Weights{Easy: 2, Medium: 4, Hard: 6})
	if got.Earned != 2+6+0.5 || got.Max != 2+4+6+3 || got.Correct != 2 {
		t.Errorf("frozen 2/4/6 weights: %+v, want earned 8.5 max 15 correct 2", got)
	}
}

func TestPercentRoundsToTwoDecimals(t *testing.T) {
	cases := []struct {
		earned, max, want float64
	}{
		{7, 9, 77.78},
		{2, 3, 66.67},
		{1, 3, 33.33},
		{1, 8, 12.5},
		{1, 32, 3.13},
		{4.5, 10, 45},
		{10, 10, 100},
		{0, 10, 0},
		{0, 0, 0},
		{5, 0, 0},
	}
	for _, tc := range cases {
		if got := percent(tc.earned, tc.max); got != tc.want {
			t.Errorf("percent(%v, %v) = %v, want %v", tc.earned, tc.max, got, tc.want)
		}
	}
}

func TestPermutationIsSeededAndStable(t *testing.T) {
	const key = "subtest-1"
	first := permutation(42, key, 8)
	if again := permutation(42, key, 8); !reflect.DeepEqual(first, again) {
		t.Errorf("same seed and key gave %v then %v", first, again)
	}
	if want := []int{0, 1, 6, 3, 2, 7, 4, 5}; !reflect.DeepEqual(first, want) {
		t.Errorf("seed 42 order = %v, want the pinned fixture %v", first, want)
	}

	sorted := append([]int{}, first...)
	sort.Ints(sorted)
	if !reflect.DeepEqual(sorted, identity(8)) {
		t.Errorf("order %v is not a permutation of 0..7", first)
	}

	if other := permutation(43, key, 8); reflect.DeepEqual(first, other) {
		t.Errorf("seeds 42 and 43 gave the same order %v", first)
	}
	if other := permutation(42, "subtest-2", 8); reflect.DeepEqual(first, other) {
		t.Errorf("keys subtest-1 and subtest-2 gave the same order %v", first)
	}
	if got := permutation(1, key, 0); len(got) != 0 {
		t.Errorf("empty permutation = %v", got)
	}
}

// The shuffle moves options, never their ids, so the points an option carries survive any order.
func TestSnapshotKeepsOptionIDsAndSubtestOrder(t *testing.T) {
	tryout := model.Tryout{RandomizeQuestions: true, RandomizeOptions: true}
	q := func(typ string, ids ...string) model.SourceQuestion {
		return model.SourceQuestion{SubtestID: uuid.Nil, QuestionID: uuid.New(), VersionID: uuid.New(), Type: typ, OptionIDs: ids}
	}
	subtest := func(position int, questions ...model.SourceQuestion) model.SourceSubtest {
		id := uuid.New()
		for i := range questions {
			questions[i].SubtestID = id
		}
		return model.SourceSubtest{ID: id, Name: "S", DurationSeconds: 60, Position: position, Questions: questions}
	}
	source := []model.SourceSubtest{
		subtest(1, q(model.QuestionPG, "a", "b", "c", "d"), q(model.QuestionPGMulti, "a", "b", "c"), q(model.QuestionIsian), q(model.QuestionTrueFalse, "benar", "salah"), q(model.QuestionPG, "w", "x", "y", "z")),
		subtest(2, q(model.QuestionPG, "a", "b")),
	}

	parts, questions := snapshot(7, tryout, source)
	if len(parts) != 2 || parts[0].Position != 1 || parts[1].Position != 2 {
		t.Fatalf("parts = %+v, want the tryout's subtest order kept", parts)
	}
	if again, againQ := snapshot(7, tryout, source); !reflect.DeepEqual(parts, again) || !reflect.DeepEqual(questions, againQ) {
		t.Error("the same seed built a different snapshot")
	}

	seen := map[uuid.UUID][]string{}
	for _, oq := range questions {
		seen[oq.QuestionID] = oq.OptionOrder
	}
	for _, st := range source {
		for _, sq := range st.Questions {
			got := append([]string{}, seen[sq.QuestionID]...)
			want := append([]string{}, sq.OptionIDs...)
			sort.Strings(got)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) && (len(got) != 0 || len(want) != 0) {
				t.Errorf("question %s options %v, want the same ids %v", sq.QuestionID, got, want)
			}
			if sq.Type == model.QuestionTrueFalse && !reflect.DeepEqual(seen[sq.QuestionID], sq.OptionIDs) {
				t.Errorf("benar_salah options were shuffled to %v", seen[sq.QuestionID])
			}
		}
	}

	positions := map[uuid.UUID]map[int]bool{}
	for _, oq := range questions {
		if positions[oq.SubtestID] == nil {
			positions[oq.SubtestID] = map[int]bool{}
		}
		positions[oq.SubtestID][oq.Position] = true
	}
	for _, st := range source {
		if len(positions[st.ID]) != len(st.Questions) {
			t.Errorf("subtest %s has positions %v for %d questions", st.ID, positions[st.ID], len(st.Questions))
		}
	}

	plain := model.Tryout{}
	_, natural := snapshot(7, plain, source)
	for i, oq := range natural[:5] {
		if oq.QuestionID != source[0].Questions[i].QuestionID {
			t.Errorf("randomize off moved question %d", i)
		}
	}
}
