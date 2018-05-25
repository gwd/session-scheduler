package main

import (
	"net/http"
	"sync"
)

// This has to be global because ServeHTTP cannot have a pointer receiver.
var lock sync.Mutex

type Middleware struct {
	handlers []http.Handler
}

func (m *Middleware) Add(handler http.Handler) {
	m.handlers = append(m.handlers, handler)
}

func (m Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Wrap the supplied ResponseWriter
	mw := NewMiddlewareResponseWriter(w)

	// HACK: Only allow a single access at a time for now
	lock.Lock()
	defer lock.Unlock()
	
	// Loop through all of the registered handlers
	for _, handler := range m.handlers {
		// Call the handler with our MiddlewareResponseWriter
		handler.ServeHTTP(mw, r)

		// If there was a write, stop processing
		if mw.written {
			return
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
