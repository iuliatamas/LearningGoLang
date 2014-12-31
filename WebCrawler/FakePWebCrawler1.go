// Learning golang, 1st exercise - parallel fake web crawler
// Almost identical with the method I found below:
// http://soniacodes.wordpress.com/2011/10/09/a-tour-of-go-69-exercise-web-crawler/

// Salt shaker method: I need 1 channel to protect map, one channel to 
// protect the prints, and 1 channel to keep track of threads waited for


package main

import (
	"fmt"
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

	mapChan := make(chan bool, 1)
	printChan := make(chan bool, 1)
	mapChan <- true
	printChan <- true

	var c2 func(string, int, Fetcher, chan bool)
	c2 = func (url string, depth int, fetcher Fetcher, ch chan bool){ 
		defer func() { ch <- true } ()
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
		
		workingThreadsChan := make(chan bool)
		<- mapChan
		workingThreads := 0
		for _, u := range urls {
			if !visited[u] {
				workingThreads ++
				visited[u] = true
				go c2(u, depth-1, fetcher, workingThreadsChan)
			}
		}
		mapChan <- true

		// wait for all waiting threads to return
		for i := 0; i<workingThreads; i++ {
			<-workingThreadsChan 
			<-printChan
			printChan <- true
		}

		<- printChan 
		printChan <- true
	}

	baseThreadChan := make(chan bool)
	go c2(url, depth, fetcher, baseThreadChan)
	<-baseThreadChan
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