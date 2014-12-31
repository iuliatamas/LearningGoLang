// Learning golang, 2nd exercise - real web crawler
// Wrote it myself inspired by the method below:
// http://soniacodes.wordpress.com/2011/10/09/a-tour-of-go-69-exercise-web-crawler/

package main
import (
	// "fmt"
	"sync"
	"net/http"
	"os"
	"io/ioutil"
	"strings"
	"math"
	"flag"
)

var DEPTH = 2
var WD = "."

func Fetch(rawUrl string) (string, error){
	resp, err := http.Get(rawUrl)
	if( err != nil ){
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	return string(body), err
}

func cleanUrl(rootUrl string, u string) string {
	if len(u) > 0 && u[len(u)-1] == '/' {
		u = u[:len(u)-1]
	}
	if !strings.HasPrefix(u, "http://") && strings.HasPrefix(rootUrl, "http://") {
		u = cleanUrl(rootUrl, rootUrl) + "/" + u 
	}
	return u
}

func CutFirstWhiteSpaces( body string ) string {
	for ; strings.HasPrefix(body, " "); {
		body = body[1:]
	}

	return body
}


// imperfect method of selecting urls from webpage
func GetUrls(rootUrl string, body string) []string {
	body = strings.ToLower(body)
	urls := make([]string, 1)

	var idx, idx2 int = 0, 0
	for {
		// fmt.Println(idx, body[idx:idx+5])
		if idx = strings.Index(body, "href"); idx == -1 {
			break
		}

		body = body[idx+len("href"):]
		body = CutFirstWhiteSpaces(body)
		if body != "" && body[0] != '='{
			continue
		}
		body = body[1:]
		body = CutFirstWhiteSpaces(body)
	

		if body == "" {
			break
		}

		// full url between quotes, or relative url
		if body[0] == '"' {
			body = body[1:]
			if idx2 = strings.IndexByte(body, '"'); idx == -1 {
				break
			}
		} else {
			// should have idx2 at *any* first whitespace or >
			idx2 = int(math.Min(float64(strings.IndexByte(body, ' ')), 
							float64(strings.IndexByte(body,'>'))))
		}

		u := cleanUrl(rootUrl, body[:idx2])
		urls = append(urls, u)
	}
	return urls
}

func Crawl(url string, depth int)  {
	visited := map[string]bool {url: true}
	var mtx sync.Mutex
	var wg sync.WaitGroup

	var crawl2 func(string, int, string) 
	crawl2 = func (url string, depth int, dirName string)  {
		// fmt.Println(url)
		os.Chdir(dirName)
		defer func(){ wg.Done() } ()
		if( depth <= 0 ){
			return 
		}

		body, err := Fetch(url)
		if( err != nil ){
			return 	
		}

		name := url
		if strings.HasPrefix(name, "http://") {
			name = name[len("http://"):]
		}
		newDir := ""
		if idx:=strings.LastIndex(name, "/"); idx != -1 {
			newDir = name[:idx]
			name = name[idx+1:]	
			os.Chdir(WD)
			os.MkdirAll(newDir, 0777)
			err := os.Chdir(newDir)
			if err != nil {
				// fmt.Printf("ERROR %v\n", err, name)
				os.Remove(newDir)
				return
			}
		} else {
			newDir = WD
		}

		file, err := os.Create(name)
		file.WriteString(body)
		file.Close()

		urls := GetUrls(url, body)
		mtx.Lock()
		for _,u := range urls{
			if !visited[u]{
				visited[u] = true
				wg.Add(1)
				go crawl2(u, depth-1, newDir)	
			}
		}
		mtx.Unlock()
		return 
	}

	wg.Add(1) 
	go crawl2(cleanUrl(url,url), depth, WD)
	wg.Wait()
}

func main(){
	args := os.Args
	nargs := len(args)

	var err error
	WD, err = os.Getwd()
	if( err != nil ){
		return
	}

	depthFlag := flag.Int("depth", 2, "depth of web crawling")
	flag.Parse()
	DEPTH = *depthFlag

	for i:=1; i<nargs; i++{
		url := args[i]
		Crawl(url, DEPTH)
	}
}
