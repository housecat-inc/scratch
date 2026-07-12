package api

//go:generate go run ../../cmd/openapi ../../docs/openapi.json

import (
	"github.com/cockroachdb/errors"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/option"
)

type Greeter interface {
	Greet(name string) (string, error)
}

type GreetRequest struct {
	Name string `json:"name"`
}

type GreetResponse struct {
	Greeting string `json:"greeting"`
}

func Register(s *fuego.Server, greeter Greeter) {
	fuego.Post(s, "/api/workflows/greet",
		func(c fuego.ContextWithBody[GreetRequest]) (GreetResponse, error) {
			body, err := c.Body()
			if err != nil {
				return GreetResponse{}, err
			}
			greeting, err := greeter.Greet(body.Name)
			if err != nil {
				return GreetResponse{}, errors.Wrap(err, "greet")
			}
			return GreetResponse{Greeting: greeting}, nil
		},
		option.Summary("Run the durable greet workflow"),
		option.Tags("workflows"),
	)
}

func Spec(path string) *openapi3.T {
	s := fuego.NewServer(fuego.WithEngineOptions(fuego.WithOpenAPIConfig(fuego.OpenAPIConfig{
		DisableMessages:  true,
		JSONFilePath:     path,
		PrettyFormatJSON: true,
	})))
	Register(s, nil)
	return s.OutputOpenAPISpec()
}
