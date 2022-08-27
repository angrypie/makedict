package makedict

import (
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
)

type Suggestion = string

type Dict interface {
	Lookup(word string) (suggestions []Suggestion)
	AddRawDict(source []byte, format string) error
	//Turn dictionary into JSON string with format { [key]: []string}
	ToJSON() []byte
	//How many words from source language
	Size() int
}

type DictSource struct {
	Url    string
	Format string
}

type GetDictSourcesFunc func(langPair string) []DictSource

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

		err = dict.AddRawDict(body, url.Format)
		if err != nil {
			return
		}
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

//implement Dict interface for MemDict
func (d MemDict) Lookup(word string) (suggestions []Suggestion) {
	for _, variant := range d.dict[word] {
		suggestions = append(suggestions, string(variant))
	}
	return
}

func (d MemDict) isVariantExist(word string, variant []byte) bool {
	for _, suggestion := range d.dict[string(word)] {
		if bytes.Equal(suggestion, variant) {
			return true
		}
	}
	return false
}

//TODO implement prasing different formats, right now noly two columns supported
func (d MemDict) AddRawDict(source []byte, format string) error {
	//parse source which is tsv file
	lines := bytes.Split(source, []byte("\n"))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		parts := bytes.Split(line, []byte("\t"))
		if len(parts) < 1 {
			return fmt.Errorf("invalid line: %s", line)
		}

		//TODO Decide to use lowercase or not
		word := string(bytes.ToLower(parts[3]))
		variant := bytes.ToLower(parts[2])

		// If variant elready exist
		if d.isVariantExist(word, variant) {
			continue
		}

		d.dict[word] = append(d.dict[word], variant)
	}

	return nil
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
