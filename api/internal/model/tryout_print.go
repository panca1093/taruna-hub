package model

// PrintTryout is the only shape the browser PDF sees. It has no field for a key, a correct option, an
// explanation or a point value, so none can reach the page.
type PrintTryout struct {
	TenantName string
	Title      string
	Subtests   []PrintSubtest
}

type PrintSubtest struct {
	Name            string
	DurationSeconds int
	Questions       []PrintQuestion
}

type PrintQuestion struct {
	Type    string
	Content string
	Options []string
}
