package makedict

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

func BenchmarkDict(b *testing.B) {
	getDictSources := func(langPair string) (sources []DictSource) {
		// for each testDictSourceUrl create DictSource and add to sources
		for _, url := range testDictSourceUrls {
			sources = append(sources, DictSource{Url: url})
		}

		return
	}

	_, err := CreateDict("por_eng", getDictSources)
	if err != nil {
		log.Fatal(err)
		return
	}
}

func TestDict(t *testing.T) {
	start := time.Now()
	getDictSources := func(langPair string) (sources []DictSource) {

		// for each testDictSourceUrl create DictSource and add to sources

		for _, url := range testDictSourceUrls {
			sources = append(sources, DictSource{Url: url})
		}

		return
	}

	dict, err := CreateDict("por_eng", getDictSources)
	if err != nil {
		t.Fatal(err)
	}

	wordsToFind := []string{"permitindo", "nos", "obras", "infelizmente", "conhecimento", "restrito", "contabilidade", "viver"}
	for _, word := range wordsToFind {
		suggestions := dict.Lookup(word)
		fmt.Printf("%s -->  %+v\n", word, suggestions)
	}

	fmt.Println("Total keys:", dict.Size())
	fmt.Println("Time Spent:", time.Since(start))
	err = dict.Export("pt_en.dic")
	if err != nil {
		t.Error(err)
	}

	score, err := ScoreDictInWordList("wordsList", dict)
	if err != nil {
		t.Error(err)
	}
	fmt.Println("WordList Score:", score)

}

func ScoreDictInWordList(wordListFile string, dict Dict) (score int, err error) {
	file, err := os.Open(wordListFile)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		word := strings.ToLower(scanner.Text())
		if dict.Exist(word) {
			score++
		}
	}
	return
}

func TestGuessFormat(t *testing.T) {
	source, err := GetRawBody("https://object.pouta.csc.fi/OPUS-Wikipedia/v1.0/dic/en-pt.dic.gz")
	if err != nil {
		t.Error(err)
	}

	languages := searchLanguageByLangPair([]string{"por", "eng"})

	languageFound := map[string]bool{"POR": false, "ENG": false}

	for _, lang := range languages {
		languageFound[lang.IsoCode639_3().String()] = true
	}
	for lang, found := range languageFound {
		if !found {
			t.Errorf("Language %s not found", lang)
		}
	}

	format, err := GuessSourceDictFormat(source, languages)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(format)
}

var testDictSourceUrls = []string{
	"https://object.pouta.csc.fi/OPUS-TildeMODEL/v2018/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-DGT/v2019/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-SciELO/v1/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-Wikipedia/v1.0/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-CAPES/v1/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-Europarl/v8/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-QED/v2.0a/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-EMEA/v3/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-TED2013/v1.1/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-Tanzil/v1/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-KDE4/v2/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-OpenSubtitles/v2018/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-GlobalVoices/v2018q4/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-bible-uedin/v1/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-News-Commentary/v16/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-ELRC_2922/v1/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-ELRC_3382/v1/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-tico-19/v2020-10-28/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-ELRA-W0246/v1/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-ELRC_2923/v1/dic/en-pt.dic.gz",
	"https://object.pouta.csc.fi/OPUS-Ubuntu/v14.10/dic/en-pt.dic.gz",
}

func TestVariantScore(t *testing.T) {
	dict := NewDict("POR", "ENG")
	//add words and variants to dict
	source := map[string][]string{
		"permitindo": {"allowing", "letting", "permitting", "allowing", "letting", "allowing"},
	}

	for word, variants := range source {
		for _, variant := range variants {
			dict.AddVariant(word, variant)
		}
	}

	suggestions := dict.Lookup("permitindo")

	if len(suggestions) != 3 {
		t.Error("Expected 3 suggestions")
	}

	for _, suggestion := range suggestions {
		if suggestion.Variant == "letting" && suggestion.Score != 2 {
			t.Error("Expected score 2 for 'letting'")
		}
		if suggestion.Variant == "allowing" && suggestion.Score != 3 {
			t.Error("Expected score 3 for 'allowing'")
		}
		if suggestion.Variant == "permitting" && suggestion.Score != 1 {
			t.Error("Expected score 1 for 'permitting'")
		}
	}

}
