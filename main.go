package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/xtgo/set"
)

type config struct {
	urlsFile string
	outFile  string
}

var (
	cfg   config
	urls  []string
	furls []string
)

func loadConfig() {
	urlsFile := flag.String("urlsFile", "urls.txt", "Input File containing URLs")
	outFile := flag.String("outFile", "results.txt", "Output File containing results")

	flag.Parse()

	cfg = config{
		urlsFile: *urlsFile,
		outFile:  *outFile,
	}

}

func exists(path string) (bool, int64, error) {
	fi, err := os.Stat(path)
	if err == nil {
		return true, fi.Size(), nil
	}
	if os.IsNotExist(err) {
		return false, int64(0), nil
	}
	return false, int64(0), err
}

func ensureFilePathExists(filepath string) error {
	value := false
	fsize := int64(0)

	for (value == false) || (fsize == int64(0)) {
		i, s, err := exists(filepath)
		if err != nil {
			log.Println("Failed to determine if the file exists or not..")
		}
		value = i
		fsize = s
	}

	log.Println(filepath+" File exists:", value)
	log.Println(filepath+" File size:", fsize)

	return nil
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func getFinalURL(url string, client *http.Client) (string, error) {

	var fu string

	resp, err := client.Get(url)
	if err != nil {
		log.Infof("Couldn't hit the URL: %v. Continuing..", err)
		return "", nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		log.Infof("403 response code for URL: %s", url)
		responseData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Can't read the response body: %v", err)
		}

		responseString := string(responseData)

		if strings.Contains(responseString, "window.location.replace") {
			log.Info("Response likely contains meta tag that replaces the Location URL")
			r := regexp.MustCompile("window.location.replace\\(\\'(.*)\\'\\)")
			match := r.FindStringSubmatch(responseString)
			fu = url + match[1]
		}

	} else if resp.StatusCode == 200 {
		fu = resp.Request.URL.String()
	}

	return fu, nil
}

func writeResultsToCsv(results []string, outputFilePath string) error {
	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		fmt.Printf("Couldn't create the output file: %v", err)
		return err
	}
	defer outputFile.Close()

	if len(results) != 0 {
		for _, str := range results {
			outputFile.WriteString(str + "\n")
		}
	} else {
		outputFile.WriteString("NA")
	}
	return nil
}

func main() {

	loadConfig()

	transport := &http.Transport{
		Dial:                (&net.Dialer{Timeout: 10 * time.Second}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}}

	client := &http.Client{Transport: transport, Timeout: time.Duration(10 * time.Second)}

	err := ensureFilePathExists(cfg.urlsFile)
	if err != nil {
		log.Fatalf("Couldn't ensure whether the URLs file exists or not: %v", err)
	}

	urls, err := readLines(cfg.urlsFile)
	if err != nil {
		log.Fatalf("Couldn't read the URLs file: %v", err)
	}

	for _, url := range urls {
		fu, err := getFinalURL(url, client)
		if err != nil {
			log.Fatalf("Couldn't get the final URL for %s: %v", url, err)
		}
		if fu != "" {
			fmt.Printf("Adding the URL to the results: %s\n", fu)
			furls = append(furls, fu)
		}
	}

	data := sort.StringSlice(furls)
	sort.Sort(data)
	n := set.Uniq(data) // Uniq returns the size of the set
	data = data[:n]     // trim the duplicate elements

	// writing the results to the outfile once everything is done
	err = writeResultsToCsv(data, cfg.outFile)
	if err != nil {
		log.Fatalf("Couldn't write to the out file: %v", err)
	}

	fmt.Println("=======================================")
	fmt.Println("Results saved to: " + cfg.outFile)
}
