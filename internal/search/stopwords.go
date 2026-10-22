package search

// stopwords say nothing about what a memory is about or where code lives:
// function words, question words and the filler of task prompts ("how does
// ... work", "add a new function that returns ..."), in English and Turkish.
// Recall and the code map both drop them from queries.
var stopwords = map[string]bool{}

func init() {
	for _, w := range Tokenize("the a an and or of to in on at for with by from into that this these those it its is are be " +
		"should must can will when all any each every some three two one new add adds added make create implement " +
		"implementation update change fix use using used return returns returning depending please also so then than " +
		"function functions method methods file files code value values number numbers call calls time " +
		"how what where which who why does do did done was were been has have had not no but if there here " +
		"we you me my our your they them their about just only very more most like need needs want work works " +
		// Turkish
		"ve veya ile ama fakat ancak ya da de ki mi mı mu mü bu şu o bunu şunu onu bunlar onlar şey şeyi " +
		"ben sen biz siz beni bana benden seni sana bizi bize sizi size için gibi kadar daha çok az en hem her hiç " +
		"bir iki üç var yok değil olarak olan olsun olur olabilir olacak oldu olduğu olmak ise eğer şimdi sonra önce " +
		"zaten hala hâlâ hep yine artık sadece bile diye nasıl neden niye ne nerede hangi kim böyle şöyle öyle " +
		"yani evet hayır lütfen tamam haber dersin yap yapalım yapar yaparsın misin istiyorum isterim gerek gerekir " +
		"gerekirse et ettim etti edelim edilmesi") {
		stopwords[w] = true
	}
}

// IsStopword reports whether a token from Tokenize carries no topic: a
// stopword, or a number of one or two digits ("#11", "v1.23"), which matches
// unrelated dates and versions far more often than it finds the right memory.
func IsStopword(t string) bool { return stopwords[t] || shortNumber(t) }

func shortNumber(t string) bool {
	if len(t) > 2 {
		return false
	}
	for _, r := range t {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ContentTerms returns the tokens that are not stopwords.
func ContentTerms(terms []string) []string {
	var out []string
	for _, t := range terms {
		if !IsStopword(t) {
			out = append(out, t)
		}
	}
	return out
}
