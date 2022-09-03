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
	"strings"

	"github.com/pemistahl/lingua-go"
)

type Suggestion = string

type Dict interface {
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
	dict = NewDict()
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
		format, err = GuessSourceDictFormat(body, searchLanguageByLangPair(langPair))
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

func searchLanguageByLangPair(langPair string) (langs []lingua.Language) {
	index := map[string]struct{}{}
	for _, part := range strings.Split(langPair, "_") {
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

type MemDict struct {
	dict map[string][][]byte
}

func NewDict() Dict {
	return MemDict{
		dict: make(map[string][][]byte),
	}
}

func (d MemDict) Size() int {
	return len(d.dict)
}

func (d MemDict) Export(filePath string) (err error) {
	f, err := os.Create(filePath)
	if err != nil {
		return
	}
	w := bufio.NewWriter(f)

	var buf bytes.Buffer
	for key, variants := range d.dict {
		for _, variant := range variants {
			buf.Write([]byte(key + "\t"))
			buf.Write(append(variant, '\n'))
			w.Write(buf.Bytes())
			buf.Reset()
		}
	}
	err = w.Flush()
	return
}

//implement Dict interface for MemDict
func (d MemDict) Lookup(word string) (suggestions []Suggestion) {
	for _, variant := range d.dict[word] {
		suggestions = append(suggestions, string(variant))
	}
	return
}

//implement Dict interface for MemDict
func (d MemDict) Exist(word string) (ok bool) {
	_, ok = d.dict[word]
	return
}

//isVariantExist is faster than Lookup due to not converting bytes to string
func (d MemDict) isVariantExist(word string, variant []byte) bool {
	for _, suggestion := range d.dict[string(word)] {
		if bytes.Equal(suggestion, variant) {
			return true
		}
	}
	return false
}

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

	for lines.Scan() {
		if shouldProcessRandomLines && rand.Intn(processEveryNthLine) != 0 {
			continue
		}
		//TODO it's files with single word columns separeted by space
		line := lines.Bytes()
		if len(line) == 0 {
			continue
		}

		parts := bytes.Split(line, []byte("\t"))
		if len(parts) < 2 {
			err = fmt.Errorf("invalid line: %s", line)
			return
		}

		err = newParts(parts)
		if err != nil {
			return
		}
	}
	return
}

func (d MemDict) AddRawDict(source []byte, format DictSourceFormat) error {
	return readDictSourceByLine(source, func(parts [][]byte) (err error) {
		//TODO Decide to use lowercase or not
		word := string(bytes.ToLower(parts[3]))
		variant := bytes.ToLower(parts[2])
		//break if word and exact variant elready exist
		if d.isVariantExist(word, variant) {
			return
		}
		//add new word and variant or append new variant if key already exist
		d.dict[word] = append(d.dict[word], variant)
		return
	})
}

func (d MemDict) ToJSON() []byte {
	str, _ := json.Marshal(d.dict)
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
