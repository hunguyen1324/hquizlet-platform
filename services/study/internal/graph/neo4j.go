// Package graph reads the existing quizlet-clone Kanji graph without modifying it.
package graph

import (
	"context"
	"errors"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Client struct {
	Driver   neo4j.DriverWithContext
	Database string
}

func Open(uri, username, password, database string) (*Client, error) {
	if uri == "" {
		return nil, nil
	}
	if username == "" || password == "" {
		return nil, errors.New("Neo4j credentials required")
	}
	d, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(username, password, ""))
	if err != nil {
		return nil, err
	}
	return &Client{d, database}, nil
}
func (c *Client) Related(ctx context.Context, chars []string) ([]string, error) {
	result, err := neo4j.ExecuteQuery(ctx, c.Driver,
		`UNWIND $chars AS char MATCH (k:Kanji {char:char})-[:READING|IS_COMPONENT_OF]-(other:Kanji)
   WHERE NOT other.char IN $chars RETURN DISTINCT other.char AS char ORDER BY char LIMIT 24`,
		map[string]any{"chars": chars}, neo4j.EagerResultTransformer,
		neo4j.ExecuteQueryWithDatabase(c.Database), neo4j.ExecuteQueryWithReadersRouting())
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, r := range result.Records {
		v, _ := r.Get("char")
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out, nil
}
