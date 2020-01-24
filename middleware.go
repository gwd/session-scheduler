package main

import (
	"net/http"
	"net/url"
	"sync"

	"github.com/julienschmidt/httprouter"

	"github.com/gwd/session-scheduler/sessions"
)

// This has to be global because ServeHTTP cannot have a pointer receiver.
var lock sync.Mutex

func RequestUser(r *http.Request) *User {
	session := sessions.RequestSession(r)
	if session == nil || session.UserID == "" {
		return nil
	}

	user, err := Event.Users.Find(UserID(session.UserID))
	if err != nil {
		panic(err)
	}
	return user
}

func RequireLogin(w http.ResponseWriter, r *http.Request) {
	query := url.Values{}
	query.Add("next", url.QueryEscape(r.URL.String()))

	http.Redirect(w, r, "/login?"+query.Encode(), http.StatusFound)
}

type Middleware struct {
	Logger http.HandlerFunc
	// Always: Available always, no login required
	Always *httprouter.Router

	// Active: Only available when the website is active, no login required
	Active *httprouter.Router

	// Logged-in users only: Will be redirected to login if no cookie detected
	UserAuth *httprouter.Router

	// Admin only: Will be 404 if the logged-in user isnt' an admin
	Admin *httprouter.Router
}

func (m Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mw := NewMiddlewareResponseWriter(w)

	// HACK: Only allow a single access at a time for now
	lock.Lock()
	defer lock.Unlock()

	if m.Logger != nil {
		m.Logger(w, r)
	}

	// First, look for public paths
	if handler, params, _ := m.Always.Lookup(r.Method, r.URL.Path); handler != nil {
		handler(mw, r, params)
		if mw.written {
			return
		}
	}

	u := RequestUser(r)

	// Then, look for paths which are available only when active, or for admins
	if handler, params, _ := m.Active.Lookup(r.Method, r.URL.Path); handler != nil {
		if Event.Active || u != nil {
			handler(mw, r, params)
		} else {
			http.Redirect(w, r, "/", http.StatusFound)
		}
		if mw.written {
			return
		}
	}

	// Then, look for 'requires login' paths
	if handler, params, _ := m.UserAuth.Lookup(r.Method, r.URL.Path); handler != nil {
		if u == nil {
			RequireLogin(mw, r)
		} else {
			handler(mw, r, params)
		}
		if mw.written {
			return
		}
	}

	// Then, look for 'admin-only' paths; only respond if we're
	// actually logged in as an admin
	if handler, params, _ := m.Admin.Lookup(r.Method, r.URL.Path); handler != nil {
		if u != nil && u.IsAdmin {
			handler(mw, r, params)
			if mw.written {
				return
			}
		}
	}

	// If no handlers wrote to the response, it’s a 404
	http.NotFound(w, r)
}

type MiddlewareResponseWriter struct {
	http.ResponseWriter
	written bool
}

func NewMiddlewareResponseWriter(w http.ResponseWriter) *MiddlewareResponseWriter {
	return &MiddlewareResponseWriter{
		ResponseWriter: w,
	}
}

func (w *MiddlewareResponseWriter) Write(bytes []byte) (int, error) {
	w.written = true
	return w.ResponseWriter.Write(bytes)
}

func (w *MiddlewareResponseWriter) WriteHeader(code int) {
	w.written = true
	w.ResponseWriter.WriteHeader(code)
}
