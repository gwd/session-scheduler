package main

import (
	"fmt"
	"log"
	"math/rand"
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

	always := NewRouter()

	always.GET("/", HandleHome)
	always.GET("/login", HandleSessionNew)
	always.POST("/login", HandleSessionCreate)

	always.GET("/robots.txt", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		http.ServeFile(w, r, "assets/robots.txt")
	})
	always.GET("/favicon.ico", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		http.ServeFile(w, r, "assets/favicon.ico")
	})
	always.ServeFiles(
		"/assets/*filepath",
		http.Dir("assets/"),
	)

	public := NewRouter()
	public.GET("/register", HandleUserNew)
	public.POST("/register", HandleUserCreate)

	public.GET("/discussion/notfound", HandleDiscussionNotFound)

	public.GET("/schedule", HandleScheduleView)

	public.GET("/list/:itype", HandleList)
	public.GET("/uid/:itype/:uid/:action", HandleUid)
	public.POST("/uid/:itype/:uid/:action", HandleUidPost)

	userAuth := NewRouter()
	userAuth.GET("/sign-out", HandleSessionDestroy)
	userAuth.GET("/discussion/new", HandleDiscussionNew)
	userAuth.POST("/discussion/new", HandleDiscussionCreate)

	admin := NewRouter()
	admin.GET("/admin/:template", HandleAdminConsole)
	admin.POST("/admin/:action", HandleAdminAction)

	admin.POST("/testaction/:action", HandleTestAction)

	middleware := Middleware{
		Logger:   LogRequest,
		Always:   always,
		Active:   public,
		UserAuth: userAuth,
		Admin:    admin,
	}

	if OptServeAddress == "" {
		// Generate a raw port between 1024 and 32768
		OptServeAddress = fmt.Sprintf("localhost:%d",
			rand.Int31n(32768-1024)+1024)
	}

	log.Printf("Listening on %s", OptServeAddress)
	log.Fatal(http.ListenAndServe(OptServeAddress, middleware))
}

// Creates a new public
func NewRouter() *httprouter.Router {
	router := httprouter.New()
	router.NotFound = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	return router
}
