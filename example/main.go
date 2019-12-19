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

func search(variable graphql.JSON) {
	client := graphql.New("http://localhost:8080/query", nil)

	req := graphql.Request{
		OperationName: "search",
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
		Variable: variable,
	}
	resp, err := client.Do(context.Background(), &req)
	if err != nil {
		log.Fatal(err)
	}

	var humans []*Human
	if err := resp.Guess("search", &humans); err != nil {
		log.Fatal(err)
	}
	prettyPrint(humans)
}

func main() {
	// success
	search(graphql.JSON{"text": "a"})

	// error
	search(nil)
}
