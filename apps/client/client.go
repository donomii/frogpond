package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gitlab.com/donomii/frogpond"
)

func SendDataPool(dpl []frogpond.DataPoint, port uint) {
	//Marshal the datapoollist to json
	json, err := json.Marshal(dpl)

	//Make an ioreader from the json
	reader := ioutil.NopCloser(bytes.NewReader(json))

	//Make a request
	req, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:%d/ponds/default", port), reader)
	if err != nil {
		log.Println("Failed to create request", err)
		return
	}

	//Get response body
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("Failed to get response", err)
		return
	}
	defer resp.Body.Close()
}

func GetDataPool(port uint) []frogpond.DataPoint {
	//Do an empty HTTP Post to the frogpond server to get the response

	//Make a request
	req, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:%d/ponds/default", port), nil)
	if err != nil {
		log.Println("Failed to create request", err)
		return nil
	}

	//Get response body
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("Failed to get response", err)
		return nil
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("Failed to read response body", err)
		return nil
	}

	//Unmarshal response body
	var data []frogpond.DataPoint
	err = json.Unmarshal(body, &data)
	if err != nil {
		log.Println("Failed to unmarshal response body", err, string(body))
		return nil
	}
	return data
}

func main() {
	var port uint
	var outputJson bool
	flag.UintVar(&port, "daemon-port", 16002, "Set the port to use for the frogpond server")
	flag.BoolVar(&outputJson, "json", false, "Print the output as json")
	flag.Parse()

	//Get first command line arg as a string
	if len(flag.Args()) < 1 {
		printHelp()
		os.Exit(1)
	}

	args := flag.Args()
	command := args[0]
	switch command {
	case "ips":
		fallthrough
	case "ip":
		//Do an empty HTTP Post to the frogpond server to get the response

		//Make a request
		req, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:%d/contact", port), nil)
		if err != nil {
			log.Println("Failed to create request", err)
			return
		}

		//Get response body
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Println("Failed to get response", err)
			return
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println("Failed to read response body", err)
			return
		}

		//Unmarshal response body
		var hosts []*frogpond.HostService
		err = json.Unmarshal(body, &hosts)
		if err != nil {
			log.Println("Failed to unmarshal response body", err)
			return
		}

		//Print ips
		for _, host := range hosts {
			fmt.Printf("%v:%v:%v last seen at %v\n", host.Name, host.Ip, host.Port, host.LastSeen)
		}

	case "dump":

		data := GetDataPool(port)

		//Print response body
		for _, dp := range data {
			if dp.Deleted {
				fmt.Printf("%s: (deleted) at %v\n", dp.Key, dp.Updated)
			} else {
				fmt.Printf("%v:%v, last updated at %v\n", string(dp.Key), string(dp.Value), dp.Updated.Format(time.RFC3339))
			}
		}
	case "set":
		fallthrough
	case "add":
		//Get key an value from command line args
		key := args[1]
		value := args[2]

		//Create frogpond data point
		dp := frogpond.DataPoint{Key: []byte(key), Value: []byte(value), Updated: time.Now()}

		//Make a temporary datapoollist
		dpl := []frogpond.DataPoint{dp}

		SendDataPool(dpl, port)
	case "delete":
		fallthrough
	case "remove":
		//Get key from command line args
		key := args[1]

		//Create frogpond data point
		dp := frogpond.DataPoint{Key: []byte(key), Deleted: true, Updated: time.Now()}
		//Make a temporary datapoollist
		dpl := []frogpond.DataPoint{dp}

		//Marshal the datapoollist to json
		json, err := json.Marshal(dpl)

		//Make an ioreader from the json
		reader := ioutil.NopCloser(bytes.NewReader(json))

		//Make a request
		req, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:%d/ponds/default", port), reader)
		if err != nil {
			log.Println("Failed to create request", err)
			return
		}

		//Get response body
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Println("Failed to get response", err)
			return
		}
		defer resp.Body.Close()
	case "fetch":
		fallthrough
	case "get":
		//Get key from command line args
		key := args[1]

		dpl := GetDataPool(port)

		//Find all prefixed keys in response
		for _, dp := range dpl {
			if strings.HasPrefix(string(dp.Key), key) {
				fmt.Println(string(dp.Value))

			}
		}

	default:
		printHelp()
	}

}

func printHelp() {
	//Print help
	fmt.Println("Usage: frogpond [command] [args]")
	fmt.Println("Options:")
	fmt.Println("  --daemon-port [port] - Set the port to use for the frogpond server")
	fmt.Println("Commands:")
	fmt.Println("  ips - Print the ips of known frogpond servers")
	fmt.Println("  dump - Dump all data from the frogpond data pool")
	fmt.Println("  add [key] [value] - Add a key value pair to the frogpond data pool")
	fmt.Println("  delete [key] - Delete a key from the frogpond data pool")
	fmt.Println("  get [key] - Get a value from the frogpond data pool")
}
