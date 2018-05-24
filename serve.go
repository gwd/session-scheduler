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

var OptServeAddress = "localhost:3000"

func handleSigs() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Block until a signal is received.
	s := <-c
	
	log.Printf("Got signal %v, shutting down...", s)
	lock.Lock()
	os.Exit(0)
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
	middleware.Add(router)
	middleware.Add(http.HandlerFunc(RequireLogin))
	middleware.Add(secureRouter)

	log.Printf("Listening on %s", OptServeAddress)
	log.Fatal(http.ListenAndServe(OptServeAddress, middleware))
}

// Creates a new router
func NewRouter() *httprouter.Router {
	router := httprouter.New()
	router.NotFound = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	return router
}
