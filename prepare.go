package makedict

import (
	"errors"
	"sync"
	"time"
)

//ErrPrepareTimout is returned when PrepareDicts took to long to finish
var ErrPrepareTimout = errors.New("timeout")

//PrepareDicts compiles dictionaries for various lanuage paris
//from provided bilingual sources.
func PrepareDicts(sources map[string][]DictSource) (dictionaries []Dict, err error) {
	errors := make(chan error, len(sources))

	var wg sync.WaitGroup

	for langPair, source := range sources {
		wg.Add(1)
		langPair, source := langPair, source
		go func() {
			defer wg.Done()
			getSource := func(langPair string) []DictSource {
				return source
			}
			dict, err := CreateDict(langPair, getSource)
			if err != nil {
				errors <- err
				return
			}
			dictionaries = append(dictionaries, dict)
		}()
	}
	//wait untill all gorutines are don or when first error occurs
	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		return
	case err := <-errors:
		return nil, err
	case <-time.After(30 * time.Second):
		return nil, ErrPrepareTimout
	}
}
