package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/drakos74/free-coin/internal/server"
	"github.com/rs/zerolog/log"
)

type TagQuery func() []string

type TargetQuery func(data map[string]interface{}) Series

type Server struct {
	name    string
	port    int
	tags    map[string]TagQuery
	targets map[string]TargetQuery
}

func NewServer(name string, port int) *Server {
	return &Server{
		name:    name,
		port:    port,
		tags:    make(map[string]TagQuery),
		targets: make(map[string]TargetQuery),
	}
}

func (s *Server) Tag(tag string, query TagQuery) *Server {
	s.tags[tag] = query
	return s
}

func (s *Server) Target(target string, query TargetQuery) *Server {
	s.targets[target] = query
	return s
}

func (s *Server) Run() {
	srv := server.NewServer(s.name, s.port).
		Add(server.Live()).
		AddRoute(server.POST, server.Data, "search", s.search).
		AddRoute(server.POST, server.Data, "tag-keys", s.keys).
		AddRoute(server.POST, server.Data, "tag-values", s.values).
		AddRoute(server.POST, server.Data, "annotations", s.annotations).
		Add(server.NewRoute(server.POST, server.Data).
			WithPath("query").
			Handler(s.query).
			AllowInterrupt().
			Create())
	go func() {
		err := srv.Run()
		if err != nil {
			panic(err.Error())
		}
	}()
}

func (s *Server) query(ctx context.Context, r *http.Request) (payload []byte, code int, err error) {
	var query Query
	_, err = server.ReadJson(r, false, &query)
	if err != nil {
		return payload, code, err
	}

	data := make([]Series, 0)
	tables := make([]Table, 0)

	for _, target := range query.Targets {
		t, ok := s.targets[target.Target]
		if !ok {
			log.Error().Str("target", target.Target).Msg("unknown target")
		}
		data = append(data, t(target.Data))
	}

	response := make([]interface{}, 0)
	for _, table := range tables {
		response = append(response, table)
	}
	for _, d := range data {
		response = append(response, d)
	}

	payload, err = json.Marshal(response)
	return
}

func (s *Server) annotations(_ context.Context, r *http.Request) (payload []byte, code int, err error) {

	// TODO : fix the annotations
	var query AnnotationQuery
	_, err = server.ReadJson(r, false, &query)
	if err != nil {
		return payload, code, err
	}

	if query.Annotation.Enable == false {
		return []byte("{}"), 400, nil
	}

	annotations := make([]AnnotationInstance, 0)

	payload, err = json.Marshal(annotations)
	return payload, code, err
}
func (s *Server) keys(_ context.Context, r *http.Request) (payload []byte, code int, err error) {
	tags := make([]Tag, len(s.tags))

	i := 0
	for k := range s.tags {
		tags[i] = Tag{
			Key:  k,
			Type: "string",
			Text: k,
		}
		i++
	}
	payload, err = json.Marshal(tags)
	return payload, code, err
}

func (s *Server) values(_ context.Context, r *http.Request) (payload []byte, code int, err error) {
	var tag Tag
	_, err = server.ReadJson(r, false, &tag)
	if err != nil {
		return payload, code, err
	}

	tq, ok := s.tags[tag.Key]

	if !ok {
		return payload, 400, fmt.Errorf("invalid key for tag: %s", tag.Key)
	}

	values := make([]Tag, 0)
	for _, value := range tq() {
		values = append(values, Tag{
			Key:  tag.Key,
			Type: "string",
			Text: value,
		})
	}

	payload, err = json.Marshal(values)
	return payload, code, err
}

func (s *Server) search(_ context.Context, r *http.Request) (payload []byte, code int, err error) {
	targets := make([]string, len(s.targets))
	i := 0
	for target := range s.targets {
		targets[i] = target
		i++
	}
	payload, err = json.Marshal(targets)
	return
}
