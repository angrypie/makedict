package makedict

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDict(t *testing.T) {
	start := time.Now()
	getDictSources := func(langPair string) []DictSource {
		return []DictSource{
			{
				Url:    "https://object.pouta.csc.fi/OPUS-OpenSubtitles/v2018/dic/en-pt.dic.gz",
				Format: "3 2",
			},
			{
				Url:    "https://object.pouta.csc.fi/OPUS-TildeMODEL/v2018/dic/en-pt.dic.gz",
				Format: "3 2",
			},
		}
	}

	dict, err := CreateDict("pt_en", getDictSources)
	if err != nil {
		t.Error(err)
	}

	wordsToFind := []string{"permitindo", "nos", "obras", "infelizmente", "conhecimento", "restrito", "contabilidade"}
	for _, word := range wordsToFind {
		suggestions := dict.Lookup(word)
		fmt.Printf("%s -->  %s\n", word, strings.Join(suggestions, " | "))
	}

	fmt.Println("Total keys:", dict.Size())
	fmt.Println("Time Spent:", time.Since(start))
}
