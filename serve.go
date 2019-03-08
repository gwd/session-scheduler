package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/julienschmidt/httprouter"
)

// URL scheme
// /register
// /login
// /uid/{disc,usr}/$uid/{view,edit}
// /new/discussion
// /new/user
// /list/discussions
// /list/users
// /admin/{console,test}
//

var OptServeAddress string

func handleSigs() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Block until a signal is received.
	s := <-c

	log.Printf("Got signal %v, shutting down...", s)
	lock.Lock()
	os.Exit(0)
}

// Generic log of all requests
func LogRequest(w http.ResponseWriter, r *http.Request) {
	// Let the request pass if we've got a user
	username := "[none]"
	if user := RequestUser(r); user != nil {
		username = user.Username
	}

	// originating ip, ip, user (if any), url
	log.Printf("%s (%s) %s %s %s",
		r.RemoteAddr,
		r.Header.Get("X-Forwarded-For"),
		username,
		r.Method,
		r.URL)
}

func serve() {
	go handleSigs()

	router := NewRouter()

	router.GET("/", HandleHome)
	router.GET("/register", HandleUserNew)
	router.POST("/register", HandleUserCreate)
	router.GET("/login", HandleSessionNew)
	router.POST("/login", HandleSessionCreate)

	router.GET("/discussion/notfound", HandleDiscussionNotFound)

	router.GET("/schedule", HandleScheduleView)

	router.GET("/list/:itype", HandleList)
	router.GET("/uid/:itype/:uid/:action", HandleUid)
	router.POST("/uid/:itype/:uid/:action", HandleUidPost)

	router.ServeFiles(
		"/assets/*filepath",
		http.Dir("assets/"),
	)

	secureRouter := NewRouter()
	secureRouter.GET("/sign-out", HandleSessionDestroy)
	secureRouter.GET("/discussion/new", HandleDiscussionNew)
	secureRouter.POST("/discussion/new", HandleDiscussionCreate)
	secureRouter.GET("/admin/:template", HandleAdminConsole)
	secureRouter.POST("/admin/:action", HandleAdminAction)

	secureRouter.POST("/testaction/:action", HandleTestAction)

	middleware := Middleware{}
	middleware.Add(http.HandlerFunc(LogRequest))
	middleware.Add(router)
	middleware.Add(http.HandlerFunc(RequireLogin))
	middleware.Add(secureRouter)

	log.Printf("Listening on %s", Event.ServeAddress)
	log.Fatal(http.ListenAndServe(Event.ServeAddress, middleware))
}

// Creates a new router
func NewRouter() *httprouter.Router {
	router := httprouter.New()
	router.NotFound = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	return router
}
