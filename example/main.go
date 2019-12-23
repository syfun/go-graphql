package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/syfun/go-graphql"
)

func prettyPrint(d interface{}) {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		fmt.Println(d)
		return
	}
	fmt.Println(string(b))
}

type Human struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func search(variables graphql.JSON) {
	client := graphql.New("http://localhost:8080/query", nil)
	searchQuery := `
query search($text: String!) {
	search(text: $text) {
		__typename
		... on Human {
			id
			name
		}
		... on Droid {
			id
			name
		}
		... on Starship {
			id
			name
		}
	}
}
`
	resp, err := client.Do(context.Background(), searchQuery, "search", variables)
	if err != nil {
		log.Fatal(err)
	}

	var humans []*Human
	if err := resp.Guess("search", &humans); err != nil {
		log.Fatal(err)
	}
	prettyPrint(humans)
}

type File struct {
	Filename string `json:"filename"`
	Mimetype string `json:"mimetype"`
	Encoding string `json:"encoding"`
}

type FakeFile struct {
	*bytes.Buffer
	name string
}

func (f *FakeFile) Name() string {
	return f.name
}

func singleUpload() {
	file := &FakeFile{bytes.NewBuffer([]byte("hello")), "test"}

	client := graphql.New("http://localhost:4000/", nil)

	query := `
mutation singleUpload ($file: Upload!) { 
	singleUpload(file: $file) { 
		filename
		mimetype
		encoding 
	} 
}
	`
	resp, err := client.SingleUpload(context.Background(), query, "singleUpload", file)
	if err != nil {
		log.Fatal(err)
	}

	var f File
	if err := resp.Guess("singleUpload", &f); err != nil {
		log.Fatal(err)
	}
	prettyPrint(f)
}

func main() {
	// success
	search(graphql.JSON{"text": "a"})

	// error
	// search(nil)

	singleUpload()
}
