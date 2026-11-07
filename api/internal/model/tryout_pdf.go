package model

// PrintTryout is the only shape the PDF renderer sees. It has no field for a key, a correct option, an
// explanation or a point value, so none can reach the HTML.
type PrintTryout struct {
	TenantName string
	Title      string
	Revision   int
	Subtests   []PrintSubtest
}

type PrintSubtest struct {
	Name            string
	DurationSeconds int
	Questions       []PrintQuestion
}

type PrintQuestion struct {
	Content string
	Options []string
}
