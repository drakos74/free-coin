package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"runtime"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/rs/zerolog/log"
)

type Action string

type Method string

const (
	Data Action = "data"
	Api  Action = "api"

	GET  Method = "GET"
	POST Method = "POST"
)

type Handler func(r *http.Request) ([]byte, int, error)

type Route struct {
	Action Action
	Path   string
	Method Method
	Exec   Handler
}

type Server struct {
	name   string
	port   int
	debug  bool
	block  api.Block
	routes []Route
}

func NewServer(name string, port int) *Server {
	return &Server{
		name:   name,
		port:   port,
		block:  api.NewBlock(),
		routes: make([]Route, 0),
	}
}

// Debug sets the server to debug mode
func (s *Server) Debug() {
	s.debug = true
}

// Add adds the given routes to the server
func (s *Server) AddRoute(method Method, action Action, path string, exec Handler) *Server {
	s.routes = append(s.routes, Route{
		Action: action,
		Path:   path,
		Method: method,
		Exec:   exec,
	})
	return s
}

// Add adds the given routes to the server
func (s *Server) Add(route ...Route) *Server {
	s.routes = append(s.routes, route...)
	return s
}

func (s *Server) handle(method Method, handler Handler) func(w http.ResponseWriter, r *http.Request) {
	// we should only handle one request per time,
	// in order to ease memory footprint.
	name := runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name()
	return func(w http.ResponseWriter, r *http.Request) {
		request := fmt.Sprintf("%s request : %s", method, name)
		s.block.Action <- api.NewAction(request).Create()
		defer func() {
			s.block.ReAction <- api.NewAction(request).Create()
		}()
		requestMethod := Method(r.Method)
		switch requestMethod {
		case method:
			b, code, err := handler(r)
			if err != nil {
				s.error(w, err)
			} else if code != http.StatusOK {
				s.code(w, b, code)
			} else {
				s.respond(w, b)
			}
		default:
			w.WriteHeader(http.StatusNotImplemented)
		}
	}
}

// Run starts the server
func (s *Server) Run() error {
	go func() {
		for action := range s.block.Action {
			log.Warn().
				Time("time", action.Time).
				Str("action", action.Name).
				Msg("started execution")
			reaction := <-s.block.ReAction
			log.Warn().
				Time("time", action.Time).
				Float64("duration", time.Since(action.Time).Seconds()).
				Str("reaction", reaction.Name).
				Msg("completed execution")
		}
	}()

	for _, route := range s.routes {
		if route.Path != "" {
			http.HandleFunc(fmt.Sprintf("/%s/%s", route.Action, route.Path), s.handle(route.Method, route.Exec))
		} else {
			http.HandleFunc(fmt.Sprintf("/%s", route.Action), s.handle(route.Method, route.Exec))
		}
	}

	log.Warn().Str("server", s.name).Int("port", s.port).Msg("starting server")
	if err := http.ListenAndServe(fmt.Sprintf(":%d", s.port), nil); err != nil {
		return fmt.Errorf("could not start storage server: %w", err)
	}
	return nil
}

func (s *Server) code(w http.ResponseWriter, b []byte, code int) {
	s.respond(w, b)
	w.WriteHeader(code)
}

func (s *Server) respond(w http.ResponseWriter, b []byte) {
	_, err := w.Write(b)
	if err != nil {
		log.Error().Err(err).Msg("could not write response")
	}
}

func (s *Server) error(w http.ResponseWriter, err error) {
	log.Error().Err(err).Msg("error for http request")
	s.code(w, []byte(err.Error()), http.StatusInternalServerError)
}

func Live() Route {
	return Route{
		Action: Data,
		Method: GET,
		Exec: func(r *http.Request) (payload []byte, code int, err error) {
			return []byte{}, 200, nil
		},
	}
}

func JsonRead(r *http.Request, debug bool, v interface{}) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if debug {
		log.Info().
			Str("url", fmt.Sprintf("%+v", r.URL)).
			Str("request", r.RequestURI).
			Str("header", fmt.Sprintf("%+v", r.Header)).
			Str("remote-address", r.RemoteAddr).
			Str("host", r.Host).
			Str("method", r.Method).
			Str("body", string(body)).
			Msg("received payload")
	}
	if len(body) > 0 {
		err = json.Unmarshal(body, v)
		if err != nil {
			return err
		}
	}
	return nil
}
