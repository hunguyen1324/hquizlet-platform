// Read-only connectivity probe. Credentials come only from environment variables.
package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/graph"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"net"
	"os"
	"time"
)

func main() {
	database := os.Getenv("NEO4J_DATABASE")
	if database == "" {
		database = "neo4j"
	}
	c, err := graph.Open(os.Getenv("NEO4J_URI"), os.Getenv("NEO4J_USERNAME"), os.Getenv("NEO4J_PASSWORD"), database)
	if err != nil || c == nil {
		fmt.Println("Neo4j: configuration missing or invalid")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	defer c.Driver.Close(context.Background())
	if err = c.Driver.VerifyConnectivity(ctx); err != nil {
		var serverError *neo4j.Neo4jError
		var dnsError *net.DNSError
		if errors.As(err, &serverError) {
			fmt.Println("Neo4j: server rejected connection:", serverError.Code)
		} else if errors.As(err, &dnsError) {
			fmt.Println("Neo4j: DNS lookup failed")
		} else {
			fmt.Printf("Neo4j: connectivity check failed (%T)\n", err)
		}
		os.Exit(1)
	}
	chars, err := c.Related(ctx, []string{"改"})
	if err != nil {
		fmt.Println("Neo4j: connected, graph read failed")
		os.Exit(1)
	}
	fmt.Printf("Neo4j: connected; read-only Kanji query succeeded (%d related characters)\n", len(chars))
}
