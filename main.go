package main

import (
	"log"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
)

func main() {
	err := Schedule.Load()
	if err != nil {
		log.Printf("Loading schedule data: %v", err)
		os.Exit(1)
	}

	serve()
}

func serve() {
	router := NewRouter()

	router.GET("/", HandleHome)
	router.GET("/register", HandleUserNew)
	router.POST("/register", HandleUserCreate)
	router.GET("/login", HandleSessionNew)
	router.POST("/login", HandleSessionCreate)

	router.GET("/discussion/notfound", HandleDiscussionNotFound)
	router.GET("/discussion/list", HandleDiscussionList)

	router.GET("/discussion/by-id/:discid/view", HandleDiscussionView)

	router.ServeFiles(
		"/assets/*filepath",
		http.Dir("assets/"),
	)

	secureRouter := NewRouter()
	secureRouter.GET("/sign-out", HandleSessionDestroy)
	secureRouter.GET("/account", HandleUserEdit)
	secureRouter.POST("/account", HandleUserUpdate)
	secureRouter.GET("/discussion/new", HandleDiscussionNew)
	secureRouter.POST("/discussion/new", HandleDiscussionCreate)

	middleware := Middleware{}
	middleware.Add(router)
	middleware.Add(http.HandlerFunc(RequireLogin))
	middleware.Add(secureRouter)

	log.Fatal(http.ListenAndServe("localhost:3000", middleware))
}

// Creates a new router
func NewRouter() *httprouter.Router {
	router := httprouter.New()
	router.NotFound = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	return router
}
