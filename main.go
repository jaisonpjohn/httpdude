package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

type stringFlags []string

func (i *stringFlags) String() string {
	var sb strings.Builder
	for _, str := range *i {
		sb.WriteString(str)
	}
	return sb.String()
}

func (i *stringFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}
func (i *stringFlags) Len() int {
	return len(*i)
}

var headersPtr stringFlags
var queryParamsPtr stringFlags
var bodyParamsPtr stringFlags

func main() {
	headers := map[string]string{"Content-Type": "application/json",
		"Accept": "application/json", "Cache-Control": "no-cache"}

	queryParams := make(map[string]string)
	bodyParams := make(map[string]string)
	var body map[string]interface{}

	urlPtr := flag.String("url", "", "url of service")
	bodyPtr := flag.String("body", "", "Http RequestBody")
	methodPtr := flag.String("method", "", "HTTP request Method")
	flag.Var(&headersPtr, "header", "HTTP Request Headers")
	flag.Var(&queryParamsPtr, "qp", "HTTP Query Params")
	flag.Var(&bodyParamsPtr, "bp", "HTTP Body Params")
	filePtr := flag.String("file", "", "input file path")
	flag.Parse()

	fmt.Println("-----INPUTS---------------")
	fmt.Println("url :", *urlPtr)
	fmt.Println("HTTP Method :", *methodPtr)
	fmt.Println("HTTP Method :", *bodyPtr)
	fmt.Println("Input File Path:", *filePtr)
	fmt.Println("tail:", flag.Args())
	fmt.Println("-------------------------")

	if *bodyPtr != "" {
		json.Unmarshal([]byte(*bodyPtr), &body)
	}

	if bodyParamsPtr.Len() > 0 {
		for _, bodyparamRaw := range bodyParamsPtr {
			bodyparamArray := strings.Split(bodyparamRaw, ":")
			bodyParams[bodyparamArray[0]] = bodyparamArray[1]
		}
		mergeBodyParams(body, bodyParams)
	}

	if queryParamsPtr.Len() > 0 {
		for _, queryparamRaw := range queryParamsPtr {
			queryparamArray := strings.Split(queryparamRaw, ":")
			queryParams[queryparamArray[0]] = queryparamArray[1]
		}
	}

	if headersPtr.Len() > 0 {
		for _, headerRaw := range headersPtr {
			headerArray := strings.Split(headerRaw, ":")
			headers[headerArray[0]] = headerArray[1]
		}
	}

	if *filePtr != "" {
		mergeFileContent(*filePtr, body, headers, queryParams, *methodPtr, *urlPtr)
	}

}

func mergeFileContent(filePath string, body map[string]interface{},
	headers map[string]string, queryParams map[string]string, methodParam string, url string) {

	bodyIndex := -1
	headerNames := make(map[string]int)
	queryParamNames := make(map[string]int)
	bodyParamNames := make(map[string]int)
	metaParamNames := make(map[string]int)

	csvFile, _ := os.Open(filePath)
	reader := csv.NewReader(bufio.NewReader(csvFile))

	line, error := reader.Read()
	fmt.Println("First LIne: " + strings.Join(line, ","))

	if error != nil {
		log.Fatal(error)
	}

	queryParamsKeys := make([]string, 0)

	for index, column := range line {
		column = strings.Trim(column, string('\uFEFF'))
		fmt.Println(column)
		if "body" == column {
			bodyIndex = index
		} else if strings.HasPrefix(column, "h-") {
			headerNames[strings.TrimPrefix(column, "h-")] = index
		} else if strings.HasPrefix(column, "bp-") {
			bodyParamNames[strings.TrimPrefix(column, "qp-")] = index
		} else if strings.HasPrefix(column, "meta-") {
			metaParamNames[strings.TrimPrefix(column, "meta-")] = index
		} else if strings.HasPrefix(column, "qp-") {
			queryParamNames[strings.TrimPrefix(column, "qp-")] = index
			queryParamsKeys = append(queryParamsKeys, strings.TrimPrefix(column, "qp-"))
		} else {
			// Only sane choice
			queryParamNames[column] = index
			queryParamsKeys = append(queryParamsKeys, column)
		}
	}

	file, err := os.Create("result.csv")
	checkError("Cannot create file", err)
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for {
		line, error := reader.Read()
		fmt.Println("LIne: " + strings.Join(line, ","))
		if error == io.EOF {
			break
		} else if error != nil {
			log.Fatal(error)
		}

		if bodyIndex > -1 {
			//bodyFromFile := make(map[string]interface{})
			json.Unmarshal([]byte(line[bodyIndex]), &body)
		}

		for key, val := range bodyParamNames {
			body[key] = line[val]
		}

		for key, val := range headerNames {
			headers[key] = line[val]
		}
		for key, val := range queryParamNames {
			queryParams[key] = line[val]
		}
		fmt.Println("Below queryParamNames")

		fmt.Println(queryParamNames)

		method := "GET"
		if methodParam != "" {
			method = methodParam
		} else if len(body) > 0 {
			method = "POST"
		}

		bodyJson, _ := json.Marshal(body)
		fmt.Println(string(bodyJson))

		req, err := http.NewRequest(method, url, bytes.NewBuffer(bodyJson))
		for key, val := range headers {
			req.Header.Set(key, val)
		}

		q := req.URL.Query()
		for key, val := range queryParams {
			q.Add(key, val)
		}
		req.URL.RawQuery = q.Encode()
		fmt.Println(req.URL.String())

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		fmt.Println("response Status:", resp.Status)
		fmt.Println("response Headers:", resp.Header)
		respBody, _ := ioutil.ReadAll(resp.Body)
		var output []string

		for _, key := range queryParamsKeys {
			output = append(output, key+"-"+queryParams[key])
		}

		output = append(output, string(respBody))
		fmt.Println("response Body:", string(respBody))

		_ = writer.Write(output)
		checkError("Cannot write to file", err)
	}
}

func mergeBodyParams(body map[string]interface{}, bodyParams map[string]string) {
	for k, v := range bodyParams {
		body[k] = v
	}
}

func checkError(message string, err error) {
	if err != nil {
		log.Fatal(message, err)
	}
}
