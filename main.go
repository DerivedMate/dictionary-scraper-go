package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/gocolly/colly"
	lru "github.com/hashicorp/golang-lru"
)

// Word : An internal representation of a word
type Word struct {
	word    string
	noun    int8
	adj     int8
	verb    int8
	phrasal int8
	adverb  int8
}

func handleErr(err error) {
	if err != nil {
		panic(err)
	}
}

func getWordsFromIndex(out chan string) {
	baseLinkerURL := "https://dictionary.cambridge.org/browse/english/%s/"
	linker := colly.NewCollector()

	// Links from a latter index to word chunks
	linker.OnHTML("a.dil.tcbd", func(e *colly.HTMLElement) {
		linker.Visit(e.Attr("href"))
	})

	// Get links to the words
	linker.OnHTML("a.tc-bd", func(e *colly.HTMLElement) {
		isIdiom, _ := regexp.MatchString("idiom", e.ChildText("span.pos"))

		if !isIdiom {
			out <- e.Attr("href")
		}
	})

	p := make([]byte, 26)
	for i := range p {
		toVisit := fmt.Sprintf(baseLinkerURL, string('a'+i))
		linker.Visit(toVisit)
	}
}

func getWordsDefs(in chan string, out chan Word) {
	c := colly.NewCollector(
		colly.Async(true),
	)

	c.OnHTML("article#page-content", func(article *colly.HTMLElement) {
		word := Word{
			word:    "",
			noun:    0,
			adj:     0,
			verb:    0,
			phrasal: 0,
			adverb:  0,
		}

		txts := article.ChildTexts(".hw.dhw")
		headWord := article.ChildTexts(".headword")
		if len(txts) > 0 {
			word.word = txts[0]
		} else if len(headWord) > 0 {
			word.word = headWord[0]
		} else {
			// Skip the word
			return
		}

		article.ForEach("span.pos.dpos", func(i int, partElem *colly.HTMLElement) {
			switch partElem.Text {
			case "noun":
				word.noun = 1
			case "adjective":
				word.adj = 1
			case "verb":
				word.verb = 1
			case "adverb":
				word.adverb = 1
			case "phrasal verb":
				word.phrasal = 1
			}
		})

		out <- word
	})

	for link := range in {
		c.Visit(link)
	}
}

func main() {
	start := time.Now()
	links := make(chan string)
	words := make(chan Word)

	file, err := os.Create("out.csv")
	if err != nil {
		panic(err)
	}

	out := bufio.NewWriter(file)
	defer file.Close()

	go getWordsFromIndex(links)
	go getWordsDefs(links, words)

	out.WriteString("word,noun,adjective,verb,phrasal verb,adverb\n")
	i := 0
	repCache, err := lru.New(50)
	handleErr(err)

	for w := range words {
		if !repCache.Contains(w.word) {
			fmt.Printf("[%v] %v\n", i, w.word)

			// !!! UPDATE THE ORDER OF PROPERTIES IN THE HEADER IF YOU CHANGE THEM HERE !!!
			out.WriteString(fmt.Sprintf("%s,%d,%d,%d,%d,%d\n", w.word, w.noun, w.adj, w.verb, w.phrasal, w.adverb))
			i++
			repCache.Add(w.word, 1)
			if i%300 == 0 {
				out.Flush()
			}

			if i == 4000 {
				fmt.Println(time.Since(start))
				return
			}
		}
	}
}
