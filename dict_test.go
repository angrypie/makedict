package makedict

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLangDetection(t *testing.T) {

}

func TestDict(t *testing.T) {
	start := time.Now()
	getDictSources := func(langPair string) []DictSource {
		return []DictSource{
			{
				Url:    "https://object.pouta.csc.fi/OPUS-OpenSubtitles/v2018/dic/en-pt.dic.gz",
				Format: DictSourceFormat{},
			},
			{
				Url:    "https://object.pouta.csc.fi/OPUS-TildeMODEL/v2018/dic/en-pt.dic.gz",
				Format: DictSourceFormat{},
			},
		}
	}

	dict, err := CreateDict("pt_en", getDictSources)
	if err != nil {
		t.Fatal(err)
	}

	wordsToFind := []string{"permitindo", "nos", "obras", "infelizmente", "conhecimento", "restrito", "contabilidade", "viver"}
	for _, word := range wordsToFind {
		suggestions := dict.Lookup(word)
		fmt.Printf("%s -->  %s\n", word, strings.Join(suggestions, " | "))
	}

	fmt.Println("Total keys:", dict.Size())
	fmt.Println("Time Spent:", time.Since(start))
	err = dict.Export("en_pt.dic")
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

var testDictSourceUrl = []string{
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
