package search

// stopwords say nothing about what a memory is about or where code lives:
// function words, question words and the filler of task prompts ("how does
// ... work", "add a new function that returns ..."). Recall and the code map
// both drop them from queries.
var stopwords = map[string]bool{}

func init() {
	for _, w := range Tokenize("the a an and or of to in on at for with by from into that this these those it its is are be " +
		"should must can will when all any each every some three two one new add adds added make create implement " +
		"implementation update change fix use using used return returns returning depending please also so then than " +
		"function functions method methods file files code value values number numbers call calls time " +
		"how what where which who why does do did done was were been has have had not no but if there here " +
		"we you me my our your they them their about just only very more most like need needs want work works") {
		stopwords[w] = true
	}
}

// IsStopword reports whether a token from Tokenize carries no topic.
func IsStopword(t string) bool { return stopwords[t] }

// ContentTerms returns the tokens that are not stopwords.
func ContentTerms(terms []string) []string {
	var out []string
	for _, t := range terms {
		if !stopwords[t] {
			out = append(out, t)
		}
	}
	return out
}
