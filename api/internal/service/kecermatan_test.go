package service

import (
	"testing"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

func validKecermatanInput() KecermatanInput {
	return KecermatanInput{
		ProgramID: uuid.New(), Title: "Kecermatan", ItemType: model.KecChar, ItemLength: 4, OptionsPerSoal: 5,
		SoalPerKolom: 50, KolomCount: 10, KolomDurationSeconds: 120, CoverFrom: "#f59e0b", CoverTo: "#ef4444",
	}
}

func TestKecermatanScalarsTable(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*KecermatanInput)
		field  string
		reason string
	}{
		{"odd kolom", func(i *KecermatanInput) { i.KolomCount = 11 }, "kolom_count", ReasonKecKolomOdd},
		{"odd kolom above the minimum", func(i *KecermatanInput) { i.KolomCount = 13 }, "kolom_count", ReasonKecKolomOdd},
		{"8 kolom", func(i *KecermatanInput) { i.KolomCount = 8 }, "kolom_count", ReasonKecKolomRange},
		{"0 kolom", func(i *KecermatanInput) { i.KolomCount = 0 }, "kolom_count", ReasonKecKolomRange},
		{"52 kolom", func(i *KecermatanInput) { i.KolomCount = 52 }, "kolom_count", ReasonKecKolomRange},
		{"49 soal", func(i *KecermatanInput) { i.SoalPerKolom = 49 }, "soal_per_kolom", ReasonKecSoal},
		{"101 soal", func(i *KecermatanInput) { i.SoalPerKolom = 101 }, "soal_per_kolom", ReasonKecSoal},
		{"length 0", func(i *KecermatanInput) { i.ItemLength = 0 }, "item_length", ReasonKecItemLength},
		{"length 6", func(i *KecermatanInput) { i.ItemLength = 6 }, "item_length", ReasonKecItemLength},
		{"2 options", func(i *KecermatanInput) { i.OptionsPerSoal = 2 }, "options_per_soal", ReasonKecOptions},
		{"9 options", func(i *KecermatanInput) { i.OptionsPerSoal = 9 }, "options_per_soal", ReasonKecOptions},
		{"zero duration", func(i *KecermatanInput) { i.KolomDurationSeconds = 0 }, "kolom_duration_seconds", ReasonKecDuration},
		{"negative duration", func(i *KecermatanInput) { i.KolomDurationSeconds = -1 }, "kolom_duration_seconds", ReasonKecDuration},
		{"a day of duration", func(i *KecermatanInput) { i.KolomDurationSeconds = 86400 }, "kolom_duration_seconds", ReasonKecDuration},
		{"blank title", func(i *KecermatanInput) { i.Title = "  " }, "title", ReasonKecTitle},
		{"no program", func(i *KecermatanInput) { i.ProgramID = uuid.Nil }, "program_id", ReasonKecProgram},
		{"short hex cover", func(i *KecermatanInput) { i.CoverTo = "#fff" }, "cover_to", ReasonKecCover},
		{"unknown type", func(i *KecermatanInput) { i.ItemType = "audio" }, "item_type", ReasonKecItemType},
		{"char with a built-in list", func(i *KecermatanInput) { i.BuiltinSources = []string{"char"} }, "package_ids", ReasonKecSources},
		{"symbol without a package", func(i *KecermatanInput) { i.ItemType = model.KecSymbol }, "package_ids", ReasonKecSources},
		{"symbol with two packages", func(i *KecermatanInput) {
			i.ItemType, i.PackageIDs = model.KecSymbol, []uuid.UUID{uuid.New(), uuid.New()}
		}, "package_ids", ReasonKecSources},
		{"shape with two packages", func(i *KecermatanInput) {
			i.ItemType, i.PackageIDs = model.KecShape, []uuid.UUID{uuid.New(), uuid.New()}
		}, "package_ids", ReasonKecSources},
		{"mix with no source at all", func(i *KecermatanInput) { i.ItemType = model.KecMix }, "package_ids", ReasonKecSources},
		{"mix with an unknown built-in", func(i *KecermatanInput) {
			i.ItemType, i.BuiltinSources = model.KecMix, []string{"emoji"}
		}, "builtin_sources", ReasonKecSources},
		{"mix with a repeated built-in", func(i *KecermatanInput) {
			i.ItemType, i.BuiltinSources = model.KecMix, []string{"char", "char"}
		}, "builtin_sources", ReasonKecSources},
	}
	for _, tc := range cases {
		input := validKecermatanInput()
		tc.mutate(&input)
		_, err := kecermatanScalars(input)
		var in *InputError
		if !asInputError(err, &in) || in.Fields[tc.field] != tc.reason {
			t.Errorf("%s: err = %v (fields %v), want %s = %s", tc.name, err, fieldsOf(in), tc.field, tc.reason)
		}
	}

	boundary := []struct {
		name   string
		mutate func(*KecermatanInput)
	}{
		{"the smallest legal test", func(i *KecermatanInput) {
			i.ItemLength, i.OptionsPerSoal, i.SoalPerKolom, i.KolomCount, i.KolomDurationSeconds = 1, 3, 50, 10, 1
		}},
		{"the largest legal test (D09)", func(i *KecermatanInput) {
			i.ItemLength, i.OptionsPerSoal, i.SoalPerKolom, i.KolomCount, i.KolomDurationSeconds = 5, 8, 100, 50, 3600
		}},
		{"a mix of built-ins only", func(i *KecermatanInput) { i.ItemType, i.BuiltinSources = model.KecMix, []string{"char", "shape"} }},
		{"a shape test with no package", func(i *KecermatanInput) { i.ItemType = model.KecShape }},
	}
	for _, tc := range boundary {
		input := validKecermatanInput()
		tc.mutate(&input)
		if _, err := kecermatanScalars(input); err != nil {
			t.Errorf("%s: %v, want it valid", tc.name, err)
		}
	}
}

func asInputError(err error, target **InputError) bool {
	if e, ok := err.(*InputError); ok {
		*target = e
		return true
	}
	return false
}

func fieldsOf(in *InputError) map[string]string {
	if in == nil {
		return nil
	}
	return in.Fields
}

func TestKecermatanPoolSizeFollowsItsSources(t *testing.T) {
	letters := kecermatanPool([]string{model.KecChar}, nil)
	if len(letters) != 26 {
		t.Errorf("char pool = %d, want 26", len(letters))
	}
	mix := kecermatanPool([]string{model.KecShape, model.KecChar, model.KecNumber}, nil)
	if len(mix) != 43 || mix[0].Kind != model.KecChar || mix[26].Kind != model.KecNumber || mix[36].Kind != model.KecShape {
		t.Errorf("mix pool = %d elements starting %v; want 43 in the fixed char, number, shape order whatever order they were named", len(mix), mix[0])
	}
	symbols := []model.KecermatanElement{{Kind: model.KecSymbol, Key: "A", Value: "A"}, {Kind: model.KecSymbol, Key: "★", Value: "★"}}
	if n := len(kecermatanPool([]string{model.KecChar}, [][]model.KecermatanElement{symbols})); n != 27 {
		t.Errorf("26 letters plus a symbol A and a star = %d, want 27: the symbol A is the letter A", n)
	}
}

func TestKecermatanEffectiveBuiltins(t *testing.T) {
	pkg := []uuid.UUID{uuid.New()}
	cases := []struct {
		in   KecermatanInput
		want string
	}{
		{KecermatanInput{ItemType: model.KecChar}, "char"},
		{KecermatanInput{ItemType: model.KecNumber}, "number"},
		{KecermatanInput{ItemType: model.KecShape}, "shape"},
		{KecermatanInput{ItemType: model.KecShape, PackageIDs: pkg}, ""},
		{KecermatanInput{ItemType: model.KecSymbol, PackageIDs: pkg}, ""},
		{KecermatanInput{ItemType: model.KecMix, BuiltinSources: []string{"shape", "char"}}, "char,shape"},
	}
	for _, tc := range cases {
		got := ""
		for i, name := range effectiveBuiltins(tc.in) {
			if i > 0 {
				got += ","
			}
			got += name
		}
		if got != tc.want {
			t.Errorf("%s with %d packages: built-ins %q, want %q", tc.in.ItemType, len(tc.in.PackageIDs), got, tc.want)
		}
	}
}
