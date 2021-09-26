package server

import (
	"context"
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
	Test Action = "test"

	GET  Method = "GET"
	POST Method = "POST"
)

type Handler func(ctx context.Context, r *http.Request) ([]byte, int, error)

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

func (s *Server) handle(method Method, interrupt bool, handler Handler) func(w http.ResponseWriter, r *http.Request) {
	// we should only handle one request per time,
	// in order to ease memory footprint.
	name := runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name()
	return func(w http.ResponseWriter, r *http.Request) {
		request := fmt.Sprintf("%s request : %s", method, name)
		ctx, cancel := context.WithCancel(context.Background())
		control := NewController(Start, name)
		if interrupt {
			control.AllowInterrupt().WithCancel(cancel)
		}
		action := api.NewSignal(request).
			WithContent(control).
			Create()
		s.block.Action <- action
		// wait for the go-ahead
		<-control.Reaction
		defer func() {
			control.Status = Finish
			action.WithContent(control)
			s.block.ReAction <- action
		}()
		requestMethod := Method(r.Method)
		switch requestMethod {
		case method:
			b, code, err := handler(ctx, r)
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
		cancel()
	}
}

// Run starts the server
func (s *Server) Run() error {
	go func() {
		actions := make(map[string]api.Signal)
		pendingActions := make([]api.Signal, 0)

		var triggerNext = func() {
			if len(actions) > 0 {
				log.Warn().Int("pending", len(actions)).Msg("execution halted")
				return
			}
			// if we have no running actions ... trigger what is pending
			newPendingActions := make([]api.Signal, 0)
			for i := 0; i < len(pendingActions); i++ {
				action := pendingActions[i]
				if i == 0 {
					log.Debug().
						Time("time", action.Time).
						Str("action", action.Name).
						Msg("started execution")
					controller := action.Content.(*Control)
					actions[action.ID] = action
					controller.Reaction <- struct{}{}
				} else {
					newPendingActions = append(newPendingActions, action)
				}
			}
			pendingActions = newPendingActions
		}

		for {
			select {
			case reaction := <-s.block.ReAction:
				if act, ok := actions[reaction.ID]; ok {
					// it s an existing action ... we probably need to close it
					log.Debug().
						Time("time", act.Time).
						Float64("duration", time.Since(act.Time).Seconds()).
						Str("reaction", act.Name).
						Msg("completed execution")
					delete(actions, reaction.ID)
					triggerNext()
					continue
				}
			case action := <-s.block.Action:
				// add current action ot pending ones ...
				pendingActions = append(pendingActions, action)
				// check if we have running actions and need to cancel any
				ctrl := action.Content.(*Control)
				for _, a := range actions {
					control := a.Content.(*Control)
					if ctrl.Type == control.Type {
						if control.Interrupt {
							control.Cancel()
							log.Warn().Str("type", control.Type).Msg("cancel execution")
							delete(actions, a.ID)
						}
					}
				}
				triggerNext()
			}
		}
	}()

	for _, route := range s.routes {
		if route.Path != "" {
			http.HandleFunc(fmt.Sprintf("/%s/%s", route.Action, route.Path), s.handle(route.Method, route.Interrupt, route.Exec))
		} else {
			http.HandleFunc(fmt.Sprintf("/%s", route.Action), s.handle(route.Method, route.Interrupt, route.Exec))
		}
	}

	log.Info().Str("server", s.name).Int("port", s.port).Msg("starting server")
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
