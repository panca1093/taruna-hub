package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

func validWebsiteLocale() model.WebsiteLocale {
	program := model.WebsiteProgram{Title: "Program", Summary: "Ringkasan"}
	return model.WebsiteLocale{
		Home: model.WebsiteHome{HeroTitle: "Judul", HeroBody: "Isi", ProgramCTA: "Program", PracticeCTA: "Latihan"},
		Programs: map[string]model.WebsiteProgram{
			"tni": program, "polri": program, "kedinasan": program,
		},
	}
}

func validWebsiteContent() model.WebsiteContent {
	return model.WebsiteContent{
		SchemaVersion: model.WebsiteSchemaVersion,
		Locales:       map[string]model.WebsiteLocale{"id": validWebsiteLocale(), "en": validWebsiteLocale()},
	}
}

func websiteFields(t *testing.T, err error) map[string]string {
	t.Helper()
	var in *InputError
	if !errors.As(err, &in) {
		t.Fatalf("want *InputError, got %v", err)
	}
	return in.Fields
}

func TestWebsiteContentValidation(t *testing.T) {
	type edit func(c *model.WebsiteContent)
	setHome := func(locale string, f func(h *model.WebsiteHome)) edit {
		return func(c *model.WebsiteContent) {
			l := c.Locales[locale]
			f(&l.Home)
			c.Locales[locale] = l
		}
	}
	setProgram := func(locale, slot string, f func(p *model.WebsiteProgram)) edit {
		return func(c *model.WebsiteContent) {
			p := c.Locales[locale].Programs[slot]
			f(&p)
			c.Locales[locale].Programs[slot] = p
		}
	}

	t.Run("valid baseline is accepted", func(t *testing.T) {
		if _, err := normalizeWebsiteContent(validWebsiteContent()); err != nil {
			t.Fatalf("err = %v", err)
		}
	})

	limits := []struct {
		name     string
		path     string
		max      int
		required bool
		set      func(s string) edit
	}{
		{"hero_title", "locales.en.home.hero_title", 90, true, func(s string) edit { return setHome("en", func(h *model.WebsiteHome) { h.HeroTitle = s }) }},
		{"hero_body", "locales.en.home.hero_body", 360, true, func(s string) edit { return setHome("en", func(h *model.WebsiteHome) { h.HeroBody = s }) }},
		{"program_cta", "locales.en.home.program_cta", 36, true, func(s string) edit { return setHome("en", func(h *model.WebsiteHome) { h.ProgramCTA = s }) }},
		{"practice_cta", "locales.en.home.practice_cta", 36, true, func(s string) edit { return setHome("en", func(h *model.WebsiteHome) { h.PracticeCTA = s }) }},
		{"title", "locales.id.programs.polri.title", 80, true, func(s string) edit { return setProgram("id", "polri", func(p *model.WebsiteProgram) { p.Title = s }) }},
		{"summary", "locales.id.programs.polri.summary", 320, true, func(s string) edit { return setProgram("id", "polri", func(p *model.WebsiteProgram) { p.Summary = s }) }},
		{"schedule", "locales.id.programs.kedinasan.schedule", 160, false, func(s string) edit {
			return setProgram("id", "kedinasan", func(p *model.WebsiteProgram) { p.Schedule = s })
		}},
		{"location", "locales.id.programs.kedinasan.location", 160, false, func(s string) edit {
			return setProgram("id", "kedinasan", func(p *model.WebsiteProgram) { p.Location = s })
		}},
		{"fees", "locales.en.programs.tni.fees", 160, false, func(s string) edit { return setProgram("en", "tni", func(p *model.WebsiteProgram) { p.Fees = s }) }},
		{"contact", "locales.en.programs.tni.contact", 160, false, func(s string) edit { return setProgram("en", "tni", func(p *model.WebsiteProgram) { p.Contact = s }) }},
	}
	for _, l := range limits {
		t.Run(l.name+" accepts the exact limit", func(t *testing.T) {
			c := validWebsiteContent()
			l.set(strings.Repeat("a", l.max))(&c)
			if _, err := normalizeWebsiteContent(c); err != nil {
				t.Fatalf("err = %v", err)
			}
		})
		t.Run(l.name+" rejects one over", func(t *testing.T) {
			c := validWebsiteContent()
			l.set(strings.Repeat("a", l.max+1))(&c)
			_, err := normalizeWebsiteContent(c)
			f := websiteFields(t, err)
			if f[l.path] != ReasonWebsiteTooLong || len(f) != 1 {
				t.Fatalf("fields = %v, want only %s", f, l.path)
			}
		})
		t.Run(l.name+" counts code points not bytes", func(t *testing.T) {
			c := validWebsiteContent()
			l.set(strings.Repeat("é🎓", l.max/2))(&c)
			if _, err := normalizeWebsiteContent(c); err != nil {
				t.Fatalf("err = %v", err)
			}
			l.set(strings.Repeat("🎓", l.max+1))(&c)
			f := websiteFields(t, func() error { _, e := normalizeWebsiteContent(c); return e }())
			if f[l.path] != ReasonWebsiteTooLong {
				t.Fatalf("fields = %v", f)
			}
		})
		t.Run(l.name+" trims before counting", func(t *testing.T) {
			c := validWebsiteContent()
			l.set("  \t" + strings.Repeat("a", l.max) + " \n")(&c)
			if _, err := normalizeWebsiteContent(c); err != nil {
				t.Fatalf("err = %v", err)
			}
		})
		if l.required {
			t.Run(l.name+" rejects blank", func(t *testing.T) {
				for _, blank := range []string{"", "   \n\t"} {
					c := validWebsiteContent()
					l.set(blank)(&c)
					_, err := normalizeWebsiteContent(c)
					f := websiteFields(t, err)
					if f[l.path] != ReasonWebsiteRequired || len(f) != 1 {
						t.Fatalf("blank %q: fields = %v", blank, f)
					}
				}
			})
		} else {
			t.Run(l.name+" accepts empty meaning information pending", func(t *testing.T) {
				c := validWebsiteContent()
				l.set("   ")(&c)
				out, err := normalizeWebsiteContent(c)
				if err != nil {
					t.Fatalf("err = %v", err)
				}
				if got := out.Locales["en"].Programs["tni"].Fees + out.Locales["id"].Programs["kedinasan"].Schedule; got != "" {
					t.Fatalf("trimmed optional = %q", got)
				}
			})
		}
	}

	t.Run("normalized output is trimmed and the input is not mutated", func(t *testing.T) {
		c := validWebsiteContent()
		setHome("id", func(h *model.WebsiteHome) { h.HeroTitle = "  Halo \n" })(&c)
		setProgram("en", "tni", func(p *model.WebsiteProgram) { p.Contact = " 0812 " })(&c)
		out, err := normalizeWebsiteContent(c)
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if out.Locales["id"].Home.HeroTitle != "Halo" || out.Locales["en"].Programs["tni"].Contact != "0812" {
			t.Fatalf("not trimmed: %+v", out.Locales["id"].Home)
		}
		if c.Locales["id"].Home.HeroTitle != "  Halo \n" {
			t.Fatal("input mutated")
		}
	})

	t.Run("HTML stays inert text", func(t *testing.T) {
		c := validWebsiteContent()
		setHome("id", func(h *model.WebsiteHome) { h.HeroTitle = `<script>alert(1)</script>` })(&c)
		out, err := normalizeWebsiteContent(c)
		if err != nil || out.Locales["id"].Home.HeroTitle != `<script>alert(1)</script>` {
			t.Fatalf("out = %q err = %v", out.Locales["id"].Home.HeroTitle, err)
		}
	})

	structural := []struct {
		name string
		edit edit
		path string
	}{
		{"wrong schema version", func(c *model.WebsiteContent) { c.SchemaVersion = 2 }, "schema_version"},
		{"zero schema version", func(c *model.WebsiteContent) { c.SchemaVersion = 0 }, "schema_version"},
		{"missing en locale", func(c *model.WebsiteContent) { delete(c.Locales, "en") }, "locales.en"},
		{"missing id locale", func(c *model.WebsiteContent) { delete(c.Locales, "id") }, "locales.id"},
		{"unknown locale", func(c *model.WebsiteContent) { c.Locales["fr"] = validWebsiteLocale() }, "locales.fr"},
		{"nil locales", func(c *model.WebsiteContent) { c.Locales = nil }, "locales.id"},
		{"missing slot", func(c *model.WebsiteContent) { delete(c.Locales["id"].Programs, "polri") }, "locales.id.programs.polri"},
		{"unknown slot", func(c *model.WebsiteContent) {
			c.Locales["en"].Programs["akpol"] = model.WebsiteProgram{Title: "x", Summary: "y"}
		}, "locales.en.programs.akpol"},
	}
	for _, tc := range structural {
		t.Run(tc.name, func(t *testing.T) {
			c := validWebsiteContent()
			tc.edit(&c)
			_, err := normalizeWebsiteContent(c)
			f := websiteFields(t, err)
			if _, ok := f[tc.path]; !ok {
				t.Fatalf("fields = %v, want key %s", f, tc.path)
			}
		})
	}

	t.Run("every failing field is reported at once", func(t *testing.T) {
		c := validWebsiteContent()
		setHome("id", func(h *model.WebsiteHome) { h.HeroTitle = "" })(&c)
		setProgram("en", "tni", func(p *model.WebsiteProgram) { p.Summary = strings.Repeat("a", 321) })(&c)
		_, err := normalizeWebsiteContent(c)
		f := websiteFields(t, err)
		if f["locales.id.home.hero_title"] != ReasonWebsiteRequired || f["locales.en.programs.tni.summary"] != ReasonWebsiteTooLong || len(f) != 2 {
			t.Fatalf("fields = %v", f)
		}
	})
}
