package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/rs/zerolog/log"
)

type Action string

type Method string

const (
	Data Action = "data"
	Api  Action = "api"
	Test Action = "test"

	GET     Method = "GET"
	POST    Method = "POST"
	OPTIONS Method = "OPTIONS"
)

type Handler func(ctx context.Context, r *http.Request) ([]byte, int, error)

func Accept(ctx context.Context, r *http.Request) ([]byte, int, error) {
	return []byte(""), 200, nil
}

type Route struct {
	Action    Action
	Interrupt bool
	Path      string
	Method    Method
	Exec      Handler
}

func NewRoute(method Method, action Action) *Route {
	return &Route{
		Action: action,
		Method: method,
		Exec: func(ctx context.Context, r *http.Request) ([]byte, int, error) {
			return []byte{}, http.StatusNotImplemented, nil
		},
	}
}

func (r *Route) Key() string {
	return fmt.Sprintf("%s_%s", r.Method, r.Path)
}

func (r *Route) AllowInterrupt() *Route {
	r.Interrupt = true
	return r
}

func (r *Route) WithPath(path string) *Route {
	r.Path = path
	return r
}

func (r *Route) Handler(exec Handler) *Route {
	r.Exec = exec
	return r
}

func (r *Route) Create() Route {
	return *r
}

type Server struct {
	name   string
	port   int
	debug  bool
	block  map[string]api.Block
	routes []Route
}

func NewServer(name string, port int) *Server {
	return &Server{
		name:   name,
		port:   port,
		block:  make(map[string]api.Block),
		routes: make([]Route, 0),
	}
}

// Debug sets the server to debug mode
func (s *Server) Debug() {
	s.debug = true
}

// AddRoute adds the given routes to the server
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

func (s *Server) handle(route Route) func(w http.ResponseWriter, r *http.Request) {
	// we should only handle one request per time,
	// in order to ease memory footprint.
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(context.Background())
		control := NewController(Start, route.Key())
		if route.Interrupt {
			control.AllowInterrupt().WithCancel(cancel)
		}
		action := api.NewSignal(route.Key()).
			WithContent(control).
			Create()

		// split logic by http method
		requestMethod := Method(r.Method)
		// enable cors
		w.Header().Set("Access-Control-Allow-Origin", "*")
		switch requestMethod {
		case route.Method:
			select {
			case s.block[route.Key()].Action <- action:
			// we re going ahead
			default:
				s.error(w, fmt.Errorf("route %s is busy", route.Key()))
				cancel()
				return
			}

			defer func() {
				control.Status = Finish
				action.WithContent(control)
				s.block[route.Key()].ReAction <- action
			}()

			b, code, err := route.Exec(ctx, r)
			if err != nil {
				s.error(w, err)
			} else if code != http.StatusOK {
				s.code(w, b, code)
			} else {
				s.respond(w, b)
			}
		case OPTIONS:
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Detail")
			s.respond(w, []byte{})
		default:
			w.WriteHeader(http.StatusNotImplemented)
		}
		cancel()
	}
}

// Run starts the server
func (s *Server) Run() error {

	for _, route := range s.routes {
		s.block[route.Key()] = api.NewBlock()
		if route.Path != "" {
			http.HandleFunc(fmt.Sprintf("/%s/%s", route.Action, route.Path), s.handle(route))
		} else {
			http.HandleFunc(fmt.Sprintf("/%s", route.Action), s.handle(route))
		}
	}

	// start the route controller
	for _, bl := range s.block {
		go func(block api.Block) {
			for {
				select {
				case action := <-block.Action:
					log.Debug().
						Str("id", action.ID).
						Str("coin", string(action.Coin)).
						Str("action", action.Name).
						Msg("started processing")
					// wait for the route to return
					reaction := <-block.ReAction
					log.Debug().
						Str("id", reaction.ID).
						Str("coin", string(reaction.Coin)).
						Str("action", reaction.Name).
						Msg("finished processing")
				}
			}
		}(bl)
	}

	log.Info().Str("server", s.name).Int("port", s.port).Msg("starting server")
	if err := http.ListenAndServe(fmt.Sprintf(":%d", s.port), nil); err != nil {
		return fmt.Errorf("could not start storage server: %w", err)
	}
	return nil
}

func (s *Server) code(w http.ResponseWriter, b []byte, code int) {
	w.WriteHeader(code)
	s.respond(w, b)
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
		Exec: func(ctx context.Context, r *http.Request) (payload []byte, code int, err error) {
			return []byte{}, 200, nil
		},
	}
}

func Read(r *http.Request, debug bool) (string, error) {
	body, err := ioutil.ReadAll(r.Body)
	if debug {
		log.Debug().
			Str("url", fmt.Sprintf("%+v", r.URL)).
			Str("request", r.RequestURI).
			Str("header", fmt.Sprintf("%+v", r.Header)).
			Str("remote-address", r.RemoteAddr).
			Str("host", r.Host).
			Str("method", r.Method).
			Str("body", string(body)).
			Msg("received payload")
	}
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func ReadJson(r *http.Request, debug bool, v interface{}) (string, error) {
	body, err := ioutil.ReadAll(r.Body)
	if debug {
		log.Debug().
			Err(err).
			Str("url", fmt.Sprintf("%+v", r.URL)).
			Str("request", r.RequestURI).
			Str("header", fmt.Sprintf("%+v", r.Header)).
			Str("remote-address", r.RemoteAddr).
			Str("host", r.Host).
			Str("method", r.Method).
			Str("body", string(body)).
			Msg("received payload")
	}

	payload := string(body)

	if err != nil {
		return payload, err
	}
	if len(body) > 0 {
		err = json.Unmarshal(body, v)
		if err != nil {
			return payload, err
		}
	}
	return payload, nil
}
