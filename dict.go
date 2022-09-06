package makedict

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/pemistahl/lingua-go"
)

type Suggestion struct {
	Variant string
	Score   int
}

type Dict interface {
	AddVariant(word string, variant string)
	Lookup(word string) (suggestions []Suggestion)
	//Exist does not convert bytes variants to string
	Exist(word string) (ok bool)
	AddRawDict(source []byte, format DictSourceFormat) error
	//Turn dictionary into JSON string with format { [key]: []string}
	ToJSON() []byte
	//How many words from source language
	Size() int
	Export(filePath string) (err error)
}

type DictSourceFormat map[string]int

type DictSource struct {
	Url string
}

type GetDictSourcesFunc func(langPair string) []DictSource

//CreateDict downloads dictionary sources and merges them together.
func CreateDict(langPair string, getUrls GetDictSourcesFunc) (dict Dict, err error) {
	langs := strings.Split(langPair, "_")
	if len(langs) != 2 {
		err = errors.New("lang pair should contain two language ISO_639_3 codes separated by _")
		return
	}
	dict = NewDict(langs[0], langs[1])
	urls := getUrls(langPair)
	//download dict sources or retrive from cache
	for _, url := range urls {
		var body []byte
		body, err = GetRawBody(url.Url)
		if err != nil {
			return
		}
		if len(body) == 0 {
			fmt.Println("ERR empty body", url.Url)
			continue
		}

		var format DictSourceFormat
		format, err = GuessSourceDictFormat(body, searchLanguageByLangPair(langs))
		if err != nil {
			err = fmt.Errorf("guessing source format for %s %w", url, err)
			fmt.Println("ERR", err)
			continue
		}

		err = dict.AddRawDict(body, format)
		if err != nil {
			return
		}
	}
	return dict, nil
}

//searchLanguageByLangPair searches lingua.Language based on ISO_639_3 codes
func searchLanguageByLangPair(codes []string) (langs []lingua.Language) {
	index := map[string]struct{}{}
	for _, part := range codes {
		index[strings.ToUpper(part)] = struct{}{}
	}

	//Search lingua.Language by IsoCode639_3code
	for _, lang := range lingua.AllLanguages() {
		if _, ok := index[lang.IsoCode639_3().String()]; !ok {
			continue
		}
		langs = append(langs, lang)
	}
	return
}

type Variant struct {
	Score int
}

type MemDict struct {
	index      map[string]map[string]Variant
	sourceLang string
	targetLang string
}

//TODO introduce type for languages to be sure that it's compatible with ISO_639_3
func NewDict(source, target string) Dict {
	return MemDict{
		index:      make(map[string]map[string]Variant),
		sourceLang: strings.ToUpper(source),
		targetLang: strings.ToUpper(target),
	}
}

func (d MemDict) Size() int {
	return len(d.index)
}

func (d MemDict) Export(filePath string) (err error) {
	f, err := os.Create(filePath)
	if err != nil {
		return
	}
	w := bufio.NewWriter(f)

	var buf strings.Builder
	const separator = "\t"
	const newLine = "\n"
	for key, variants := range d.index {

		//convert variants map to slice
		var suggestions []Suggestion
		for variant, v := range variants {
			suggestions = append(suggestions, Suggestion{
				Variant: variant,
				Score:   v.Score,
			})
		}

		//make most popular word appear first
		slices.SortFunc(suggestions, func(i, j Suggestion) bool {
			return i.Score > j.Score
		})

		buf.WriteString(key)
		for _, v := range suggestions {
			buf.WriteString(separator)
			buf.WriteString(v.Variant)
			buf.WriteString(separator)
			buf.WriteString(strconv.Itoa(v.Score))
		}
		buf.WriteString(newLine)

		w.WriteString(buf.String())
		buf.Reset()
	}
	err = w.Flush()
	return
}

//implement Dict interface for MemDict
func (d MemDict) Lookup(word string) (suggestions []Suggestion) {
	for value, variant := range d.index[word] {
		suggestions = append(suggestions, Suggestion{
			Variant: value,
			Score:   variant.Score,
		})
	}
	return
}

//implement Dict interface for MemDict
func (d MemDict) Exist(word string) (ok bool) {
	_, ok = d.index[word]
	return
}

//isVariantExist is faster than Lookup due to not converting bytes to string
func (d MemDict) VariantExist(word string, variant string) (ok bool) {
	variants, ok := d.index[word]
	if !ok {
		return
	}
	_, ok = variants[variant]
	return
}

func (d MemDict) AddVariant(word string, variant string) {
	variants, ok := d.index[word]
	if !ok {
		variants = make(map[string]Variant)
		d.index[word] = variants
	}

	variants[variant] = Variant{Score: variants[variant].Score + 1}
}

//read dictionary source line by line ignoring blank lines and not column based format
//starts from tsv with tabs as separators but falls back to spaces if tabs not detected
func readDictSourceByLine(
	source []byte, newParts func(parts [][]byte) error, processRandomLines ...int,
) (err error) {
	reader := bytes.NewReader(source)
	lines := bufio.NewScanner(reader)
	lines.Split(bufio.ScanLines)

	shouldProcessRandomLines := len(processRandomLines) > 0
	processEveryNthLine := 0
	if shouldProcessRandomLines {
		processEveryNthLine = processRandomLines[0]
	}

	lineSeparator := []byte("\t")
	for lines.Scan() {
		if shouldProcessRandomLines && rand.Intn(processEveryNthLine) != 0 {
			continue
		}
		//TODO it's files with single word columns separeted by space
		line := lines.Bytes()
		if len(line) == 0 {
			continue
		}

		parts := bytes.Split(line, lineSeparator)
		if len(parts) < 2 {
			parts = bytes.Split(line, []byte(" "))
			if len(parts) < 2 {
				err = fmt.Errorf("invalid line: %s", line)
				return
			}
			lineSeparator = []byte(" ")
		}

		err = newParts(parts)
		if err != nil {
			return
		}
	}
	return
}

func (d MemDict) AddRawDict(source []byte, format DictSourceFormat) error {
	targetColumn := format[d.targetLang]
	sourceColumn := format[d.sourceLang]
	return readDictSourceByLine(source, func(parts [][]byte) (err error) {
		//TODO Decide to use lowercase or not
		word := string(bytes.ToLower(parts[sourceColumn]))
		variant := string(bytes.ToLower(parts[targetColumn]))
		//TODO source dictionary still contains words on from different launguages
		//should we bother to remove them for now, it's maybe like 10% of the space?
		//add new word and variant or append new variant if key already exist
		d.AddVariant(word, variant)
		return
	})
}

func (d MemDict) ToJSON() []byte {
	str, _ := json.Marshal(d.index)
	return str
}

//GetRawBody retrive from cache or makes GET request to obtail raw dict source
func GetRawBody(url string) (body []byte, err error) {
	body, err = ReadCache(url)
	if err != nil {
		return
	}
	//return cached body if exist
	if body != nil {
		return
	}

	body, err = GetRawBodyHTTP(url)
	if err != nil {
		return
	}
	err = WriteCache(url, body)
	if err != nil {
		return
	}
	return
}

//GetRawBodyHTTP mae GET request to url and returns body as a []bytes
func GetRawBodyHTTP(url string) (body []byte, err error) {
	fmt.Println("INFO loading dic tsource from", url)
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	tr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return
	}
	body, err = ioutil.ReadAll(tr)
	return
}

func WriteCache(url string, content []byte) (err error) {
	sum := getCacheFileName(url)
	f, err := os.Create(sum)
	if err != nil {
		return
	}
	defer f.Close()

	_, err = f.Write(content)
	return
}

func getCacheFileName(url string) string {
	sum := sha1.Sum([]byte(url))
	return fmt.Sprintf("source_cache_%x", sum)
}

func ReadCache(url string) (content []byte, err error) {
	sum := getCacheFileName(url)
	content, err = ioutil.ReadFile(sum)
	if errors.Is(err, fs.ErrNotExist) {
		err = nil
		return
	}
	return
}

//try to guess format of dict source which is tsv file
func GuessSourceDictFormat(sourceDict []byte, langs []lingua.Language) (format DictSourceFormat, err error) {
	if len(langs) < 0 {
		err = fmt.Errorf("no languages provided")
		return
	}
	score := make(map[lingua.Language][]int)
	//fore each lang create entry in score map
	for _, lang := range langs {
		score[lang] = []int{}
	}

	detector := lingua.NewLanguageDetectorBuilder().FromLanguages(langs...).Build()

	err = readDictSourceByLine(sourceDict, func(parts [][]byte) (err error) {
		for columnNumber, part := range parts {
			language, exists := detector.DetectLanguageOf(string(part))
			if !exists {
				continue
			}
			//add column number to score for each detecetd language
			score[language] = append(score[language], columnNumber)
		}
		return
	}, 100) //process only every 100th line, source dictionaries usually contains tousands
	if err != nil {
		return
	}

	//loop over score map and find the lang with the most columns
	format = DictSourceFormat{}
	columnExist := make(map[int]struct{})
	for lang, columns := range score {
		column := mostFrequentNumber(columns)
		//two languages cant have the same column number
		if _, ok := columnExist[column]; ok {
			err = errors.New("same column detected for different language")
			return
		}
		columnExist[column] = struct{}{}
		format[lang.IsoCode639_3().String()] = column
	}
	return
}

//function that finds most frequent number in array
func mostFrequentNumber(arr []int) (num int) {
	num = 0
	m := make(map[int]int)
	for _, v := range arr {
		m[v]++
		if m[v] > m[num] {
			num = v
		}
	}
	return
}
