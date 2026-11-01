package service

import (
	"strings"
	"unicode/utf8"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

const (
	ReasonWebsiteRequired = "website_required"
	ReasonWebsiteTooLong  = "website_too_long"
	ReasonWebsiteSchema   = "website_schema_version"
	ReasonWebsiteLocale   = "website_locale"
	ReasonWebsiteSlot     = "website_program_slot"
)

var (
	websiteLocales = []string{"id", "en"}
	websiteSlots   = []string{"tni", "polri", "kedinasan"}
)

// websiteContent collects field errors while it trims and measures each text.
type websiteContent struct {
	fields map[string]string
}

func (w *websiteContent) text(path, value string, max int, required bool) string {
	value = strings.TrimSpace(value)
	switch {
	case required && value == "":
		w.fields[path] = ReasonWebsiteRequired
	case utf8.RuneCountInString(value) > max:
		w.fields[path] = ReasonWebsiteTooLong
	}
	return value
}

// normalizeWebsiteContent trims every text and validates the whole content; it never mutates its argument.
func normalizeWebsiteContent(content model.WebsiteContent) (model.WebsiteContent, error) {
	w := websiteContent{fields: map[string]string{}}
	if content.SchemaVersion != model.WebsiteSchemaVersion {
		w.fields["schema_version"] = ReasonWebsiteSchema
	}
	out := model.WebsiteContent{SchemaVersion: content.SchemaVersion, Locales: map[string]model.WebsiteLocale{}}

	for key := range content.Locales {
		if key != "id" && key != "en" {
			w.fields["locales."+key] = ReasonWebsiteLocale
		}
	}
	for _, key := range websiteLocales {
		locale, ok := content.Locales[key]
		if !ok {
			w.fields["locales."+key] = ReasonWebsiteRequired
			continue
		}
		out.Locales[key] = w.locale("locales."+key, locale)
	}

	if len(w.fields) > 0 {
		return model.WebsiteContent{}, &InputError{Fields: w.fields}
	}
	return out, nil
}

func (w *websiteContent) locale(prefix string, in model.WebsiteLocale) model.WebsiteLocale {
	home := prefix + ".home."
	out := model.WebsiteLocale{
		Home: model.WebsiteHome{
			HeroTitle:   w.text(home+"hero_title", in.Home.HeroTitle, 90, true),
			HeroBody:    w.text(home+"hero_body", in.Home.HeroBody, 360, true),
			ProgramCTA:  w.text(home+"program_cta", in.Home.ProgramCTA, 36, true),
			PracticeCTA: w.text(home+"practice_cta", in.Home.PracticeCTA, 36, true),
		},
		Programs: map[string]model.WebsiteProgram{},
	}

	for slot := range in.Programs {
		if slot != "tni" && slot != "polri" && slot != "kedinasan" {
			w.fields[prefix+".programs."+slot] = ReasonWebsiteSlot
		}
	}
	for _, slot := range websiteSlots {
		p, ok := in.Programs[slot]
		path := prefix + ".programs." + slot
		if !ok {
			w.fields[path] = ReasonWebsiteRequired
			continue
		}
		out.Programs[slot] = model.WebsiteProgram{
			Title:    w.text(path+".title", p.Title, 80, true),
			Summary:  w.text(path+".summary", p.Summary, 320, true),
			Schedule: w.text(path+".schedule", p.Schedule, 160, false),
			Location: w.text(path+".location", p.Location, 160, false),
			Fees:     w.text(path+".fees", p.Fees, 160, false),
			Contact:  w.text(path+".contact", p.Contact, 160, false),
		}
	}
	return out
}
