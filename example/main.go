package main

import (
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

func main() {
	client := graphql.New("http://localhost:8080/query", nil)

	req := graphql.Request{
		OperationName: "",
		Query: `query search($text: String!) {
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
}`,
		Variable: graphql.JSON{"text": "a"},
	}
	resp, err := client.Do(context.Background(), &req)
	if err != nil {
		log.Fatal(err)
	}
	prettyPrint(resp)

	var humans []*Human
	if err := resp.Guess("search", &humans); err != nil {
		log.Fatal(err)
	}
	prettyPrint(humans)
}
