// Learning golang, 1st exercise - parallel fake web crawler
// Almost identical with the method I found below:
// http://soniacodes.wordpress.com/2011/10/09/a-tour-of-go-69-exercise-web-crawler/

// Using: 1 capacity one channel gor protecting print
// a mutex for protecting the map, waitgroup for waiting on the threads

package main

import (
	"fmt"
	"sync"
)

type Fetcher interface {
	// Fetch returns the body of URL and
	// a slice of URLs found on that page.
	Fetch(url string) (body string, urls []string, err error)
}


// Crawl uses fetcher to recursively crawl
// pages starting with url, to a maximum of depth.
func Crawl(url string, depth int, fetcher Fetcher) {
	// var visited map[string]bool = make(map[string]bool) // also ok
	visited := map[string]bool {url: true}
	visited[url] = true

	var mtx  sync.Mutex
	var wg  sync.WaitGroup
	printChan := make(chan bool, 1)
	printChan <- true

	var c2 func(string, int, Fetcher)
	c2 = func (url string, depth int, fetcher Fetcher){ 
		defer wg.Done()
		if depth <= 0 {
			return
		}
		body, urls, err := fetcher.Fetch(url)
		if err != nil {
			<- printChan
			fmt.Println(err)
			printChan <- true
			return
		}
		<- printChan
		fmt.Printf("found: %s %q\n", url, body)
		printChan <- true
		
		mtx.Lock()
		for _, u := range urls {
			if !visited[u] {
				visited[u] = true
				wg.Add(1)
				go c2(u, depth-1, fetcher)
			}
		}
		mtx.Unlock()
	}

	wg.Add(1)
	go c2(url, depth, fetcher)
	wg.Wait()
	fmt.Println("got last true out")

	return
}

func main() {
	Crawl("http://golang.org/", 4, fetcher)
}

// fakeFetcher is Fetcher that returns canned results.
type fakeFetcher map[string]*fakeResult

type fakeResult struct {
	body string
	urls []string
}

func (f fakeFetcher) Fetch(url string) (string, []string, error) {
	if res, ok := f[url]; ok {
		return res.body, res.urls, nil
	}
	return "", nil, fmt.Errorf("not found: %s", url)
}

// fetcher is a populated fakeFetcher.
var fetcher = fakeFetcher{
	"http://golang.org/": &fakeResult{
		"The Go Programming Language",
		[]string{
			"http://golang.org/pkg/",
			"http://golang.org/cmd/",
		},
	},
	"http://golang.org/pkg/": &fakeResult{
		"Packages",
		[]string{
			"http://golang.org/",
			"http://golang.org/cmd/",
			"http://golang.org/pkg/fmt/",
			"http://golang.org/pkg/os/",
		},
	},
	"http://golang.org/pkg/fmt/": &fakeResult{
		"Package fmt",
		[]string{
			"http://golang.org/",
			"http://golang.org/pkg/",
		},
	},
	"http://golang.org/pkg/os/": &fakeResult{
		"Package os",
		[]string{
			"http://golang.org/",
			"http://golang.org/pkg/",
		},
	},
}