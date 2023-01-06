package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"

	"regexp"
	"strings"
)

type Result struct {
	Events      []Event  `json:"events"`
	Courses     []Course `json:"courses"`
	TotalStarts int      `json:"totalStarts"`
}

type Event struct {
	Name   string `json:"name"`
	Place  string `json:""`
	Starts int    `json:"starts"`
}

type Course struct {
	Name   string `json:"name"`
	Starts int    `json:"starts"`
}

var count map[string]int
var course map[string]int

var result Result

func main() {

	year := os.Args[1]

	count = make(map[string]int)
	course = make(map[string]int)

	program, err := http.Get("https://raw.githubusercontent.com/Skarmjakten/website/main/data/program/" + year + ".yaml")

	for i := 1; i < 25; i++ {
		doc := strconv.Itoa(i)
		if i < 10 {
			doc = "0" + doc
		}
		resp, err := http.Get("https://www.skarmjakten.fi/" + year + "/" + doc + "_res.html")
		if err != nil {
			continue
		}

		parseResp(resp, strconv.Itoa(i))
	}

	for k, v := range count {
		fmt.Printf("Club: %s. Starts: %v\n", k, v)
	}

	for k, v := range course {
		fmt.Printf("Course: %s. Starts: %v\n", k, v)
		c := Course{
			Name:   k,
			Starts: v,
		}
		result.Courses = append(result.Courses, c)
	}

	fmt.Printf("Total: %v\n", result.TotalStarts)
	data, _ := json.Marshal(result)
	ioutil.WriteFile("result.json", data, 0755)

}

func parseResp(resp *http.Response, name string) {
	b := resp.Body
	defer b.Close()

	z := html.NewTokenizer(b)
	eventTotal := 0
	defer func() {
		result.Events = append(result.Events, Event{
			Name:   name,
			Starts: eventTotal,
		})
	}()
	for {

		tt := z.Next()
		switch {
		case tt == html.ErrorToken:
			return
		case tt == html.StartTagToken:
			t := z.Token()
			//select course
			if strings.ToLower(t.Data) == "h3" {
				z.Next()
				courseName := parseCourseName(z)

				countC := 0
			course:
				for {
					tt = z.Next()
					t = z.Token()
					switch {
					case tt == html.ErrorToken:
						return
					case tt == html.StartTagToken:
						if strings.ToLower(t.Data) == "pre" {
							tt = z.Next()
							countC = parseCourse(z.Token())
							result.TotalStarts += countC
							eventTotal += countC

							if val, ok := course[courseName]; ok {
								val += countC
								course[courseName] = val
							} else {
								course[courseName] = countC
							}

							break course
						}
					}
				}
			}
		}
	}

}

func parseCourseName(z *html.Tokenizer) string {
	t := z.Token()
	if t.Data == "a" {
		z.Next()
		t = z.Token()
	}
	data := t.Data

	reg := regexp.MustCompile("[0-9]([.,])?([0-9])?")
	km := regexp.MustCompile("km([.])?")
	b := regexp.MustCompile("[bB]ana([-])?")

	bana := reg.ReplaceAllString(data, "")
	bana = km.ReplaceAllString(bana, "")
	bana = b.ReplaceAllString(bana, "")
	bana = strings.ReplaceAll(bana, "-", "")
	bana = strings.TrimSpace(bana)
	fmt.Printf("Bana: '%v' from '%v'\n", bana, data)
	return bana
}

func parseCourse(z html.Token) int {

	courseCount := 0
	buf := bytes.NewBufferString(z.Data)
	reader := bufio.NewReader(buf)
	reg := regexp.MustCompile("^[1-9].|^-")
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			break
		}
		li := strings.TrimLeft(string(line), " ")
		isResult := reg.FindString(string(li))
		if isResult != "" {
			addCount(count, string(line))
			courseCount++
		}
	}

	return courseCount
}

func addCount(m map[string]int, line string) {

	club := strings.ToLower(AwkColumn(line, 4))
	r := regexp.MustCompile("^[a-zA-Z]")
	if club != "" {
		if r.FindString(club) == "" {
			club = "Ingen förening"
		}

		if val, ok := m[club]; ok {
			val++
			m[club] = val
		} else {
			m[club] = 1
		}
	}
}

// AwkEndOfLine gets the full text til end of line start from column.
func AwkEndOfLine(text string, fromCol int) string {
	awkProg := "{ for(i=" + strconv.Itoa(fromCol) + "; i<=NF; i++) { print($i); print ' ' }}"

	return executeAwk(text, awkProg)
}

func AwkColumn(text string, column int) string {
	awkProg := "$0 { print $" + strconv.Itoa(column) + " }"

	return executeAwk(text, awkProg)
}

func executeAwk(text string, awkProg string) string {
	input := bytes.NewReader([]byte(text))
	buf := bytes.NewBuffer([]byte{})
	config := &interp.Config{
		Stdin:  input,
		Output: buf,
		Vars:   []string{"ORS", ""},
	}
	prog, err := parser.ParseProgram([]byte(awkProg), nil)
	if err != nil {
		fmt.Println(err)
		return ""
	}

	_, err = interp.ExecProgram(prog, config)
	if err != nil {
		log.Errorf("Failed to awk: %v", err)
	}

	return buf.String()
}
